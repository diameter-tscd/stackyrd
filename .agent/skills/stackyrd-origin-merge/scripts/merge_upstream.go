package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	dryRun         = flag.Bool("dry-run", false, "Show what would be done without making changes")
	branch         = flag.String("branch", "development", "Target branch name")
	upstreamBranch = flag.String("upstream-branch", "upstream/development", "Upstream branch to merge from")
	nanoRef        = flag.String("nano-ref", "origin/base/nano", "Nano reference branch for patches")
	verbose        = flag.Bool("verbose", false, "Verbose output")
)

var deletePaths = []string{
	"internal/services/modules/",
	"pkg/plugin/",
	"pkg/metrics/",
	"docs/",
	"scripts/plugin/",
	"scripts/swagger/",
	"tests/services/",
	"tests/infrastructure/afero_test.go",
	"tests/infrastructure/testdata/",
	"pkg/testing/",
	"deployments/",
}

var deleteFiles = []string{
	"internal/middleware/swagger.go",
	"pkg/infrastructure/afero.go",
	"pkg/infrastructure/cron_manager.go",
	"pkg/infrastructure/grafana.go",
	"pkg/infrastructure/kafka.go",
	"pkg/infrastructure/minio.go",
	"pkg/infrastructure/mongo.go",
	"pkg/infrastructure/redis.go",
	"pkg/utils/image.go",
	"PLUGIN_GUIDE.md",
	"versioninfo.json",
}

var stripPrefixes = []string{
	"pkg/infrastructure/kafka",
	"pkg/infrastructure/mongo",
	"pkg/infrastructure/minio",
	"pkg/infrastructure/grafana",
	"pkg/infrastructure/redis",
	"pkg/infrastructure/cron",
	"pkg/infrastructure/afero",
}

func log(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
}

func verboseLog(format string, a ...interface{}) {
	if *verbose {
		log(format, a...)
	}
}

func run(cmd string, args ...string) (string, error) {
	verboseLog("running: %s %s", cmd, strings.Join(args, " "))
	c := exec.Command(cmd, args...)
	c.Stderr = os.Stderr
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

func runOutput(cmd string, args ...string) (string, error) {
	if *dryRun {
		return "", nil
	}
	return run(cmd, args...)
}

func runSilent(cmd string, args ...string) error {
	if *dryRun {
		return nil
	}
	c := exec.Command(cmd, args...)
	return c.Run()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isDeletedByNano checks if a path should be deleted (is underneath a delete path or matches a delete file).
func isDeletedByNano(path string) bool {
	for _, dp := range deletePaths {
		dp = strings.TrimSuffix(dp, "/")
		if path == dp || strings.HasPrefix(path, dp+"/") {
			return true
		}
	}
	for _, df := range deleteFiles {
		if path == df {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()

	if err := runMerge(); err != nil {
		log("ERROR: %v", err)
		os.Exit(1)
	}
}

func runMerge() error {
	log("=== Stackyrd → Nano Merge ===")

	if *dryRun {
		log(">>> DRY RUN — no changes will be made <<<")
	}

	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("must run from project root (no go.mod found)")
	}

	// Step 1: Fetch upstream
	log("\n[1/8] Fetching upstream...")
	if _, err := runOutput("git", "fetch", "upstream"); err != nil {
		return fmt.Errorf("fetch upstream failed: %w", err)
	}
	log("upstream fetched")

	// Step 2: Check upstream branch
	nanoRef := *nanoRef
	upRef := *upstreamBranch
	log("\n[2/8] Checking upstream branch %s...", upRef)
	if _, err := run("git", "rev-parse", "--verify", upRef); err != nil {
		upRef = "remotes/" + *upstreamBranch
		if _, err := run("git", "rev-parse", "--verify", upRef); err != nil {
			return fmt.Errorf("upstream branch %q not found", *upstreamBranch)
		}
	}

	// Step 3: Create branch from upstream
	log("\n[3/8] Creating branch %q from %s...", *branch, upRef)
	if _, err := run("git", "rev-parse", "--verify", "refs/heads/"+*branch); err == nil {
		log("branch %q exists, deleting...", *branch)
		// If we're on this branch, switch to detached HEAD first
		cur, _ := run("git", "rev-parse", "--abbrev-ref", "HEAD")
		if cur == *branch {
			runSilent("git", "checkout", "--detach", upRef)
		}
		if err := runSilent("git", "branch", "-D", *branch); err != nil {
			return fmt.Errorf("failed to delete existing branch %q: %w", *branch, err)
		}
	}
	if _, err := runOutput("git", "checkout", "-b", *branch, upRef); err != nil {
		return fmt.Errorf("failed to create branch %q: %w", *branch, err)
	}
	log("on branch %s", *branch)

	// Step 4: Delete stripped directories and files
	log("\n[4/8] Stripping deleted files and directories...")
	for _, dir := range deletePaths {
		d := strings.TrimSuffix(dir, "/")
		if fileExists(d) {
			log("  rm -rf %s", d)
			if err := runSilent("git", "rm", "-rf", d); err != nil {
				log("  WARNING: failed to remove %s: %v", d, err)
			}
		} else {
			verboseLog("  skip (not found): %s", d)
		}
	}
	for _, f := range deleteFiles {
		if fileExists(f) {
			log("  rm %s", f)
			if err := runSilent("git", "rm", f); err != nil {
				log("  WARNING: failed to remove %s: %v", f, err)
			}
		} else {
			verboseLog("  skip (not found): %s", f)
		}
	}

	// Step 5: Apply nano patches from the nano ref
	// Get all files from the nano ref and apply each one that:
	//   (a) is not a file that should be deleted (per delete paths/files),
	//   (b) exists in the nano ref.
	log("\n[5/8] Applying nano patches...")

	nanoFilesRaw, err := run("git", "ls-tree", "-r", "--name-only", nanoRef)
	if err != nil {
		return fmt.Errorf("failed to list nano ref files: %w", err)
	}
	nanoFiles := strings.Split(nanoFilesRaw, "\n")

	patchCount := 0
	skipCount := 0
	for _, f := range nanoFiles {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if isDeletedByNano(f) {
			skipCount++
			continue
		}
		verboseLog("  checkout %s from %s", f, nanoRef)
		if *dryRun {
			log("  would checkout %s from %s", f, nanoRef)
		} else {
			if out, err := run("git", "checkout", nanoRef, "--", f); err != nil {
				log("  WARNING: failed to checkout %s: %s", f, out)
				continue
			}
			run("git", "add", f)
		}
		patchCount++
	}
	log("patched %d files from nano (%d skipped — deleted by nano)", patchCount, skipCount)

	// Step 6: Strip new upstream files matching strip patterns
	log("\n[6/8] Checking for new upstream files matching strip patterns...")
	newStripped := 0
	for _, prefix := range stripPrefixes {
		matches, _ := filepath.Glob(prefix + "*")
		for _, match := range matches {
			if fileExists(match) {
				log("  rm (new upstream): %s", match)
				if err := runSilent("git", "rm", match); err != nil {
					log("  WARNING: failed to remove %s: %v", match, err)
				}
				newStripped++
			}
		}
	}
	if isDir("internal/services/modules") {
		entries, _ := os.ReadDir("internal/services/modules")
		for _, e := range entries {
			if !e.IsDir() {
				path := "internal/services/modules/" + e.Name()
				log("  rm (new upstream service): %s", path)
				if err := runSilent("git", "rm", path); err != nil {
					log("  WARNING: failed to remove %s: %v", path, err)
				}
				newStripped++
			}
		}
	}
	log("stripped %d new upstream files", newStripped)

	// Step 7: go mod tidy
	log("\n[7/8] Tidying modules...")
	if *dryRun {
		log("  would run: go mod tidy")
	} else {
		if err := runSilent("go", "mod", "tidy"); err != nil {
			log("  WARNING: go mod tidy failed: %v (may need manual fixup)", err)
		} else {
			run("git", "add", "go.mod", "go.sum")
			log("  go.mod tidied")
		}
	}

	// Step 8: Commit
	log("\n[8/8] Committing...")
	if *dryRun {
		log("  would commit")
	} else {
		out, _ := run("git", "status", "--porcelain")
		if out == "" {
			log("  nothing to commit — no changes")
		} else {
			msg := fmt.Sprintf("merge: sync with upstream %s into nano", *upstreamBranch)
			if err := runSilent("git", "commit", "-m", msg); err != nil {
				return fmt.Errorf("commit failed: %w", err)
			}
			log("  committed: %s", msg)
		}
	}

	log("\n=== Merge Summary ===")
	log("Branch: %s (based on %s)", *branch, upRef)
	log("Nano ref: %s", nanoRef)
	log("Files patched from nano: %d", patchCount)
	log("Files skipped (deleted by nano): %d", skipCount)
	log("New upstream files stripped: %d", newStripped)

	if *dryRun {
		log("\n>>> DRY RUN — no changes were made <<<")
	} else {
		log("\nNext steps:")
		log("  Verify build: go build ./cmd/app")
		log("  Run tests:    go test ./...")
		log("  Push:         git push origin %s", *branch)
	}

	return nil
}
