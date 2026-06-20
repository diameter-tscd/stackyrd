package plugin

type RouteMethod string

const (
	RouteGET    RouteMethod = "GET"
	RoutePOST   RouteMethod = "POST"
	RoutePUT    RouteMethod = "PUT"
	RouteDELETE RouteMethod = "DELETE"
	RoutePATCH  RouteMethod = "PATCH"
	RouteWS     RouteMethod = "WS"
)

type RouteDefinition struct {
	Path         string
	Method       RouteMethod
	Handler      string
	Public       bool
	StaticDir    string
	StaticPrefix string
	StaticIndex  string
}

type RouteRegistrarPlugin interface {
	PluginRoutes() []RouteDefinition
}

type scriptRoutePlugin struct {
	Plugin
	routes []RouteDefMeta
}

func (s *scriptRoutePlugin) PluginRoutes() []RouteDefinition {
	result := make([]RouteDefinition, len(s.routes))
	for i, r := range s.routes {
		result[i] = RouteDefinition{
			Path:       r.Path,
			Method:     RouteMethod(r.Method),
			Handler:    r.Handler,
			Public:     r.Public,
			StaticDir:  r.StaticDir,
			StaticPrefix: r.StaticPrefix,
			StaticIndex:  r.StaticIndex,
		}
	}
	return result
}
