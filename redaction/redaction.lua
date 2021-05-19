
--[[
Copyright 2021 Couchbase, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file  except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the  License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
--]]

--[[ Simple method to redact information in-flight with a pure LUA implementation.
    The recommendation is to add a worker thread for LUA processing.
    This can be used by fluent bit LUA filter then:

    [FILTER]
        Name    lua
        Match   raw
        script  redaction.lua
        call    cb_sub_message

    This uses the separate SHA1 hashing library from https://github.com/mpeterv/sha1
    which must be installed and available on package.path.
--]]
function cb_sub_message(tag, timestamp, record)
    -- Iterate over possible keys to redact (HTTP is the main cause of this)
    keys = {'message', 'host', 'user', 'method', 'path', 'code', 'size', 'client'}
    changed = false
    for i, current_key in ipairs(keys) do
        -- Overwrite original key with redacted version
        original = record[current_key]
        if original then
            -- no support for case sensitivity unfortunately so ensure we lowercase the search tags
            lowered = string.gsub(original, "</*UD>", string.lower)
            --[[
            We need to extract the string between <ud>..</ud> tags and then
            hash it before re-inserting it between the tags again.

            Cats are <ud>sma#@&*+-.!!!!!rter</ud> than dogs, and <ud>sheeps</ud>
            Cats are <ud>d18b681a1966c325d736effe7036ef26</ud> than dogs, and <ud>6bce3e2226016eb568e822b09f0ee020</ud>
            --]]
            sha1_redacted = string.gsub(lowered, "<ud>(.-)</ud>", cb_hash_string )
            record[current_key] = sha1_redacted
            changed = true
        end 
    end

    -- Indicate whether we updated it or not
    if changed then 
        return 2, 0, record
    end
    return 0, 0, record
end

-- Provide the contents of a file or empty string
-- Copes with a dynamic salt then as well (if you really want one)
function cb_read_file_contents(file)
    local f, err = io.open(file, "rb")
    if not f then
        return ""
    end
    local content = f:read("*all")
    f:close()
    return content
end

-- Deal with hashing and surrounding with our tags
function cb_hash_string(input)
    local sha1 = require "sha1"
    if input then
        -- Grab the salt if we have one from a config file in the usual config area (so it can be provided as a secret volume mounted)
        local salt = cb_read_file_contents( "/fluent-bit/config/redaction.salt" )
        -- Now hash everything with the salt
        hash_output = sha1.sha1(salt .. input)
        return "<ud>" .. hash_output .. "</ud>" 
    end
    return input
end