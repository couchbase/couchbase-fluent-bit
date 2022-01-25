-- [FILTER]
--     Name        modify
--     Match       couchbase.log.*
--     Condition   Key_Exists                              pod['namespace']
--     Condition   Key_Exists                              couchbase['cluster']
--     Condition   Key_Exists                              couchbase['node']
-- Set         couchbase.node   $couchbase['node'].$couchbase['cluster'].$pod['namespace']

function set_hostname(tag, timestamp, record)
    new_record=record
    if record["pod"] and record["couchbase"] then
        pod = record["pod"]
        couchbase = record["couchbase"]
        if pod["namespace"] and couchbase["cluster"] and couchbase["node"] then
            new_record["couchbase"]["node"] = couchbase["node"] .. "." .. couchbase["cluster"] .. "." .. pod['namespace'] .. ".svc"
        end
    end
    return 2, 0, new_record
end