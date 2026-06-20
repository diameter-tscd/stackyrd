#!/usr/bin/env python3
"""Plugin host that runs a Python plugin as a gRPC server.

Usage:
    python3 host.py --socket /tmp/plugin.sock --name myplugin

The host reads the plugin script source from the gRPC Execute request,
loads it, and executes it. The SDK (sdk.py) is auto-discovered from
the same directory as this host script.
"""

import argparse
import base64
import importlib.util
import json
import marshal
import os
import sys
import threading
import time

import grpc

import plugin_pb2
import plugin_pb2_grpc

# Ensure the SDK directory is on sys.path so loaded plugins can do:
#   from sdk import Plugin
_host_dir = os.path.dirname(os.path.abspath(__file__))
if _host_dir not in sys.path:
    sys.path.insert(0, _host_dir)


class PluginHostServicer(plugin_pb2_grpc.PluginRuntimeServicer):
    """gRPC servicer that loads and executes Python plugins."""

    def __init__(self, name: str):
        self.name = name
        self._module_lock = threading.Lock()
        self._module = None
        self._execution_count = 0
        self._last_error = ""

    def _load_module(self, source: str, module_id: str = ""):
        if self._module is not None:
            return self._module
        spec = importlib.util.spec_from_loader("plugin_loader", None)
        module = importlib.util.module_from_spec(spec)
        # Set __file__ so relative path lookups work inside plugins
        module.__file__ = os.path.join(_host_dir, f"{self.name}_plugin.py")
        module.__name__ = f"plugins.{self.name}"
        module.__package__ = "plugins"
        if source.startswith("PYC:"):
            bytecode = base64.b64decode(source[4:])
            code = marshal.loads(bytecode)
            exec(code, module.__dict__)
        else:
            exec(source, module.__dict__)
        self._module = module
        return module

    def _find_plugin_class(self, module):
        """Find the first class in the module that is a Plugin subclass."""
        for attr_name in dir(module):
            attr = getattr(module, attr_name)
            if not isinstance(attr, type):
                continue
            if attr_name == "Plugin":
                continue
            if hasattr(attr, "execute") and callable(getattr(attr, "execute")):
                # Verify it's actually overridden (not the base class method)
                base_exec = getattr(attr, "execute", None)
                if base_exec is not None:
                    return attr
        return None

    def Execute(self, request, context):
        self._execution_count += 1
        try:
            module = self._load_module(request.script_source)

            args = {}
            if request.args_json:
                args = json.loads(request.args_json)

            plugin_class = self._find_plugin_class(module)
            if plugin_class is None:
                return plugin_pb2.ExecuteResponse(
                    success=False,
                    error=f"No plugin class with execute() found in {self.name}"
                )

            plugin_instance = plugin_class()
            if hasattr(plugin_instance, "configure"):
                plugin_instance.configure(self.name)

            # If the plugin has the new SDK lifecycle, use run();
            # otherwise fall back to direct execute() call.
            if hasattr(plugin_instance, "run"):
                result = plugin_instance.run(args)
            else:
                try:
                    raw = plugin_instance.execute(args)
                except Exception as e:
                    return plugin_pb2.ExecuteResponse(
                        success=False,
                        error=f"{type(e).__name__}: {e}",
                    )
                if raw is None:
                    result = {"success": True, "data": None}
                elif isinstance(raw, dict):
                    result = raw
                else:
                    result = {"success": True, "data": raw}

            if result is None:
                return plugin_pb2.ExecuteResponse(success=True, data_json=b"null")

            if isinstance(result, dict):
                return plugin_pb2.ExecuteResponse(
                    success=result.get("success", True),
                    data_json=json.dumps(result.get("data", {})).encode(),
                    error=result.get("error", ""),
                )

            return plugin_pb2.ExecuteResponse(
                success=True,
                data_json=json.dumps(result).encode(),
            )

        except Exception as e:
            self._last_error = str(e)
            return plugin_pb2.ExecuteResponse(
                success=False,
                error=f"{type(e).__name__}: {e}",
            )

    def Ping(self, request, context):
        return plugin_pb2.Pong(
            version="1.0.0",
        )


def main():
    parser = argparse.ArgumentParser(description="Python plugin host")
    parser.add_argument("--socket", required=True, help="Unix socket path")
    parser.add_argument("--name", default="plugin", help="Plugin name")
    parser.add_argument("--script", help="Path to plugin script (optional)")
    args = parser.parse_args()

    server = grpc.server(thread_pool=threading.ThreadPoolExecutor(max_workers=4))
    servicer = PluginHostServicer(args.name)
    plugin_pb2_grpc.add_PluginRuntimeServicer_to_server(servicer, server)

    if os.path.exists(args.socket):
        os.unlink(args.socket)

    server.add_insecure_port(f"unix:{args.socket}")
    server.start()

    # Signal readiness to the Go side
    print("READY", flush=True)

    # Pre-load a script if provided via --script
    if args.script and os.path.exists(args.script):
        with open(args.script) as f:
            source = f.read()
        servicer._load_module(source)

    server.wait_for_termination()


if __name__ == "__main__":
    main()
