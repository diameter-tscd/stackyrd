"""SDK for writing Python plugins.

Usage:
    from sdk import Plugin

    class MyPlugin(Plugin):
        def execute(self, args: dict) -> dict:
            name = args.get("name", "world")
            return {
                "success": True,
                "data": {"message": f"Hello, {name}!"}
            }

Lifecycle:
    Plugin lifecycle follows: __init__() -> configure() -> setup() -> execute() -> teardown()
    setup() and teardown() are called on every execution unless skipped by the plugin.
"""

import json
import logging
import sys
import time
import traceback


class Logger:
    """Simple structured logger for plugins."""

    def __init__(self, name: str, level: str = "INFO"):
        self._name = name
        self._level = getattr(logging, level.upper(), logging.INFO)
        self._logger = logging.getLogger(f"plugin.{name}")
        self._logger.setLevel(self._level)
        if not self._logger.handlers:
            handler = logging.StreamHandler(sys.stdout)
            handler.setFormatter(logging.Formatter(
                "[%(levelname)s] plugin.%(name)s: %(message)s"
            ))
            self._logger.addHandler(handler)

    def debug(self, msg: str, **kwargs):
        if kwargs:
            self._logger.debug("%s %s", msg, json.dumps(kwargs))
        else:
            self._logger.debug(msg)

    def info(self, msg: str, **kwargs):
        if kwargs:
            self._logger.info("%s %s", msg, json.dumps(kwargs))
        else:
            self._logger.info(msg)

    def warn(self, msg: str, **kwargs):
        if kwargs:
            self._logger.warning("%s %s", msg, json.dumps(kwargs))
        else:
            self._logger.warning(msg)

    def error(self, msg: str, **kwargs):
        if kwargs:
            self._logger.error("%s %s", msg, json.dumps(kwargs))
        else:
            self._logger.error(msg)


class Plugin:
    """Base class for all Python plugins.

    Subclass this and override execute(). Optionally override
    setup() and teardown() for lifecycle hooks.

    Instance attributes available to subclasses:
        self.name       (str)   — Plugin name, set by configure()
        self.logger     (Logger) — Structured logger
        self.state      (dict)  — Persistent state across executions
        self.cache      (dict)  — In-memory cache (cleared on plugin reload)
    """

    def __init__(self):
        self.name = ""
        self.logger = None
        self.state = {}
        self.cache = {}
        self._elapsed = 0.0

    def configure(self, name: str):
        """Called before the first execute to set the plugin name."""
        self.name = name
        self.logger = Logger(name)

    def setup(self, args: dict) -> None:
        """Called before each execute(). Override for pre-execution init.

        Raise an exception to abort execution.
        """
        pass

    def execute(self, args: dict) -> dict:
        """Override this method to implement plugin logic.

        Args:
            args: Dictionary of execution arguments

        Returns:
            dict with standard keys:
                success (bool): Whether execution succeeded
                data (any): Result data (will be JSON-serialized)
                error (str, optional): Error message if failed
        """
        raise NotImplementedError("Subclasses must implement execute()")

    def teardown(self, args: dict, result: dict) -> None:
        """Called after each execute(). Override for post-execution cleanup.

        Args:
            args: The original execution arguments
            result: The result dict returned by execute()
        """
        pass

    def run(self, args: dict) -> dict:
        """Lifecycle-aware execution wrapper.

        Calls setup() -> execute() -> teardown() in order.
        Returns a properly structured result dict.
        """
        start = time.time()
        try:
            self.setup(args)
            result = self.execute(args)
            if result is None:
                result = {"success": True, "data": None}
            elif isinstance(result, dict) and "success" not in result:
                result = {"success": True, "data": result}
            return result
        except Exception as e:
            self.logger.error(
                "Execution failed", error=str(e), traceback=traceback.format_exc()
            )
            return {
                "success": False,
                "error": f"{type(e).__name__}: {e}",
                "data": {"traceback": traceback.format_exc()},
            }
        finally:
            elapsed = time.time() - start
            self._elapsed = elapsed
            try:
                self.teardown(args, result if "result" in dir() else {})
            except Exception:
                pass

    def elapsed_ms(self) -> float:
        return self._elapsed * 1000.0

    def get_state(self, key: str, default=None):
        return self.state.get(key, default)

    def set_state(self, key: str, value):
        self.state[key] = value

    def clear_state(self):
        self.state.clear()
