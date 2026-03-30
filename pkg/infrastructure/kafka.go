package infrastructure

import (
	"context"
	"fmt"
	"stackyard/config"
	"stackyard/pkg/logger"

	"github.com/IBM/sarama"
)

type KafkaManager struct {
	Producer sarama.SyncProducer
	Brokers  []string
	GroupID  string
	logger   *logger.Logger
	Pool     *WorkerPool // Async worker pool
}

// Name returns the display name of the component
func (k *KafkaManager) Name() string {
	return "Kafka"
}

func NewKafkaManager(cfg config.KafkaConfig, logger *logger.Logger) (*KafkaManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to start kafka producer: %w", err)
	}

	// Initialize worker pool for async operations
	pool := NewWorkerPool(5) // Fewer workers for Kafka (producer heavy)
	pool.Start()

	return &KafkaManager{
		Producer: producer,
		Brokers:  cfg.Brokers,
		GroupID:  cfg.GroupID,
		logger:   logger,
		Pool:     pool,
	}, nil
}

func (k *KafkaManager) GetStatus() map[string]interface{} {
	stats := make(map[string]interface{})
	if k == nil {
		stats["connected"] = false
		return stats
	}

	if k.Producer == nil && len(k.Brokers) == 0 {

		stats["connected"] = false
		return stats
	}

	stats["connected"] = true // Assuming connected if initialized for now, complex to check liveness without producing
	stats["brokers"] = k.Brokers
	stats["group_id"] = k.GroupID
	return stats
}

// Consume starts a consumer group for the given topic.
// NOTE: This blocks the calling goroutine. Run in a separate goroutine.
func (k *KafkaManager) Consume(ctx context.Context, topic string, handler func(key, value []byte) error) error {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumerGroup, err := sarama.NewConsumerGroup(k.Brokers, k.GroupID, config)
	if err != nil {
		return fmt.Errorf("error creating consumer group: %w", err)
	}
	defer consumerGroup.Close()

	consumer := &consumerHandler{
		handler: handler,
		logger:  k.logger,
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := consumerGroup.Consume(ctx, []string{topic}, consumer); err != nil {
				return fmt.Errorf("error from consumer: %w", err)
			}
		}
	}
}

// consumerHandler implements sarama.ConsumerGroupHandler
type consumerHandler struct {
	handler func(key, value []byte) error
	logger  *logger.Logger
}

func (h *consumerHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		if err := h.handler(message.Key, message.Value); err != nil {
			h.logger.Error("Error handling message", err)
		}
		session.MarkMessage(message, "")
	}
	return nil
}

// Async Kafka Operations

// PublishAsync asynchronously publishes a message to a topic.
func (k *KafkaManager) PublishAsync(ctx context.Context, topic string, message []byte) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, k.Publish(ctx, topic, message)
	})
}

// PublishWithKeyAsync asynchronously publishes a message with a key to a topic.
func (k *KafkaManager) PublishWithKeyAsync(ctx context.Context, topic string, key, message []byte) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, k.PublishWithKey(ctx, topic, key, message)
	})
}

// PublishBatchAsync asynchronously publishes multiple messages to a topic.
func (k *KafkaManager) PublishBatchAsync(ctx context.Context, topic string, messages [][]byte) *BatchAsyncResult[struct{}] {
	operations := make([]AsyncOperation[struct{}], len(messages))

	for i, message := range messages {
		message := message // Capture loop variable
		operations[i] = func(ctx context.Context) (struct{}, error) {
			return struct{}{}, k.Publish(ctx, topic, message)
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// PublishBatchWithKeysAsync asynchronously publishes multiple messages with keys.
func (k *KafkaManager) PublishBatchWithKeysAsync(ctx context.Context, topic string, keyValuePairs [][2][]byte) *BatchAsyncResult[struct{}] {
	operations := make([]AsyncOperation[struct{}], len(keyValuePairs))

	for i, kv := range keyValuePairs {
		kv := kv // Capture loop variable
		operations[i] = func(ctx context.Context) (struct{}, error) {
			return struct{}{}, k.PublishWithKey(ctx, topic, kv[0], kv[1])
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// ConsumeAsync starts consuming messages asynchronously.
// This method starts the consumer in a goroutine and returns immediately.
func (k *KafkaManager) ConsumeAsync(ctx context.Context, topic string, handler func(key, value []byte) error) {
	k.SubmitAsyncJob(func() {
		if err := k.Consume(ctx, topic, handler); err != nil {
			k.logger.Error("Async consumer error", err, "topic", topic)
		}
	})
}

// Sync Methods (for backward compatibility and internal use)

func (k *KafkaManager) Publish(ctx context.Context, topic string, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}

	_, _, err := k.Producer.SendMessage(msg)
	return err
}

func (k *KafkaManager) PublishWithKey(ctx context.Context, topic string, key, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(message),
	}

	_, _, err := k.Producer.SendMessage(msg)
	return err
}

// Worker Pool Operations

// SubmitAsyncJob submits an async job to the worker pool.
func (k *KafkaManager) SubmitAsyncJob(job func()) {
	if k.Pool != nil {
		k.Pool.Submit(job)
	} else {
		// Fallback to direct execution if pool not available
		go job()
	}
}

// Close closes the Kafka manager and its worker pool.
func (k *KafkaManager) Close() error {
	if k.Pool != nil {
		k.Pool.Close()
	}
	if k.Producer != nil {
		return k.Producer.Close()
	}
	return nil
}

func init() {
	RegisterComponent("kafka", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Kafka.Enabled {
			return nil, nil
		}
		return NewKafkaManager(cfg.Kafka, log)
	})
}
