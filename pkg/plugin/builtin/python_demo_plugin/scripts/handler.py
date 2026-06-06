"""Demo Python plugin - echos back a greeting."""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", "..", "scripts", "plugins", "python"))
from sdk import Plugin


class GreeterPlugin(Plugin):
    def execute(self, args):
        name = args.get("name", "world")
        return {
            "success": True,
            "data": {
                "message": f"Hello from Python, {name}!",
                "source": "python-demo",
                "plugin_name": self.name,
            }
        }
