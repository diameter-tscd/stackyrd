-- Lua Demo Plugin
--
-- Echos back a greeting with the provided name.
-- Demonstrates basic Lua plugin structure: handle() function + done() callback.

function handle(args)
    local name = args["name"] or "world"

    logger:info("Greeting " .. name)

    done({
        success = true,
        data = {
            message = "Hello from Lua, " .. name .. "!",
            plugin_name = plugin_name,
            name_length = string.len(name)
        }
    })
end
