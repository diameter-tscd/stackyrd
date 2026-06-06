-- Lua Transformer Plugin
--
-- Transforms data payloads with field mapping, filtering, sorting,
-- flattening, and formatting. Multiple modes of operation.

function handle(args)
    local mode = args["mode"] or "map"

    if mode == "map" then
        handle_map(args)
    elseif mode == "filter" then
        handle_filter(args)
    elseif mode == "sort" then
        handle_sort(args)
    elseif mode == "flatten" then
        handle_flatten(args)
    elseif mode == "format" then
        handle_format(args)
    else
        done({ success = false, error = "Unknown mode: " .. tostring(mode) })
    end
end

--- Map mode: transform fields with functions
function handle_map(args)
    local data = args["data"] or {}
    local mappings = args["mappings"] or {}
    local result = {}

    for _, m in ipairs(mappings) do
        local from = m["from"]
        local to = m["to"] or from
        local fn = m["fn"] or "copy"

        local value = resolve(data, from)
        if value ~= nil then
            value = apply(value, fn, m)
            setpath(result, to, value)
        elseif m["default"] ~= nil then
            setpath(result, to, m["default"])
        end
    end

    done({ success = true, data = {
        result = result,
        field_count = table_len(mappings)
    }})
end

--- Filter mode: filter records by conditions
function handle_filter(args)
    local records = args["records"] or {}
    local conditions = args["conditions"] or {}
    local logic = args["logic"] or "and"

    if #records == 0 then
        done({ success = true, data = { records = {}, total = 0 } })
        return
    end

    local filtered = {}
    for _, record in ipairs(records) do
        local matches = {}
        for _, cond in ipairs(conditions) do
            table.insert(matches, evaluate(record, cond))
        end
        if apply_logic(matches, logic) then
            table.insert(filtered, record)
        end
    end

    done({ success = true, data = {
        records = filtered,
        total = #records,
        filtered = #records - #filtered
    }})
end

--- Sort mode: sort records by one or more fields
function handle_sort(args)
    local records = args["records"] or {}
    local sort_by = args["sort_by"] or {}
    local order = args["order"] or "asc"

    if #records == 0 or #sort_by == 0 then
        done({ success = true, data = { records = records, total = #records } })
        return
    end

    table.sort(records, function(a, b)
        for _, field in ipairs(sort_by) do
            local va = resolve(a, field)
            local vb = resolve(b, field)
            if va ~= vb then
                if order == "desc" then
                    return va > vb
                else
                    return va < vb
                end
            end
        end
        return false
    end)

    done({ success = true, data = {
        records = records,
        total = #records,
        sort_by = sort_by,
        order = order
    }})
end

--- Flatten mode: flatten nested tables into dot-notation keys
function handle_flatten(args)
    local data = args["data"] or {}
    local prefix = args["prefix"] or ""
    local sep = args["separator"] or "."

    local flat = {}
    flatten(data, flat, prefix, sep)

    done({ success = true, data = {
        flat = flat,
        key_count = table_len(flat)
    }})
end

--- Format mode: format values with pattern templates
function handle_format(args)
    local data = args["data"] or {}
    local pattern = args["pattern"] or "{value}"
    local fields = args["fields"] or {}

    local formatted = {}
    for _, field in ipairs(fields) do
        local value = resolve(data, field)
        if value ~= nil then
            local out = string.gsub(pattern, "{value}", tostring(value))
            out = string.gsub(out, "{upper}", string.upper(tostring(value)))
            out = string.gsub(out, "{lower}", string.lower(tostring(value)))
            out = string.gsub(out, "{len}", tostring(string.len(tostring(value))))
            formatted[field] = out
        end
    end

    done({ success = true, data = {
        formatted = formatted,
        fields_count = table_len(formatted)
    }})
end

--- Helpers ---

function resolve(obj, path)
    if obj == nil then return nil end
    local parts = split(path, ".")
    local current = obj
    for _, part in ipairs(parts) do
        if type(current) ~= "table" then return nil end
        current = current[part]
        if current == nil then return nil end
    end
    return current
end

function setpath(obj, path, value)
    local parts = split(path, ".")
    local current = obj
    for i = 1, #parts - 1 do
        if current[parts[i]] == nil then
            current[parts[i]] = {}
        end
        current = current[parts[i]]
    end
    current[parts[#parts]] = value
end

function apply(value, fn, mapping)
    if fn == "copy" then
        return value
    elseif fn == "upper" then
        return string.upper(tostring(value))
    elseif fn == "lower" then
        return string.lower(tostring(value))
    elseif fn == "trim" then
        return trim(tostring(value))
    elseif fn == "prefix" then
        return (mapping["value"] or "") .. tostring(value)
    elseif fn == "suffix" then
        return tostring(value) .. (mapping["value"] or "")
    elseif fn == "round" then
        local places = mapping["places"] or 0
        local mult = 10 ^ places
        return math.floor(value * mult + 0.5) / mult
    elseif fn == "abs" then
        return math.abs(value)
    elseif fn == "tostring" then
        return tostring(value)
    elseif fn == "tonumber" then
        return tonumber(value)
    else
        return value
    end
end

function evaluate(record, cond)
    local field = cond["field"]
    local op = cond["op"] or "eq"
    local expected = cond["value"]
    local actual = resolve(record, field)

    if op == "exists" then return actual ~= nil end
    if op == "not_exists" then return actual == nil end
    if op == "eq" then return actual == expected end
    if op == "neq" then return actual ~= expected end
    if op == "gt" then return actual ~= nil and expected ~= nil and actual > expected end
    if op == "gte" then return actual ~= nil and expected ~= nil and actual >= expected end
    if op == "lt" then return actual ~= nil and expected ~= nil and actual < expected end
    if op == "lte" then return actual ~= nil and expected ~= nil and actual <= expected end
    if op == "contains" then
        if type(actual) == "string" then
            return string.find(actual, tostring(expected), 1, true) ~= nil
        end
        return false
    end
    if op == "prefix" then
        if type(actual) == "string" then
            return string.sub(actual, 1, string.len(tostring(expected))) == tostring(expected)
        end
        return false
    end
    return false
end

function apply_logic(matches, logic)
    if #matches == 0 then return true end
    if logic == "and" then
        for _, m in ipairs(matches) do if not m then return false end end
        return true
    elseif logic == "or" then
        for _, m in ipairs(matches) do if m then return true end end
        return false
    elseif logic == "not" then
        for _, m in ipairs(matches) do if m then return false end end
        return true
    end
    return false
end

function flatten(obj, result, prefix, sep)
    if type(obj) == "table" then
        for k, v in pairs(obj) do
            local key = k
            if prefix ~= "" then
                key = prefix .. sep .. k
            end
            if type(v) == "table" then
                flatten(v, result, key, sep)
            else
                result[key] = v
            end
        end
    else
        result[prefix] = obj
    end
end

function trim(s)
    return string.gsub(s, "^%s*(.-)%s*$", "%1")
end

function split(s, sep)
    local parts = {}
    if s == "" then return { "" } end
    for part in string.gmatch(s, "[^" .. sep .. "]+") do
        table.insert(parts, part)
    end
    return parts
end

function table_len(t)
    local count = 0
    for _ in pairs(t) do count = count + 1 end
    return count
end
