BEGIN;

CREATE TABLE tasks (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type text NOT NULL CHECK (type IN ('uppercase', 'count_words', 'sleep')),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    result jsonb,
    error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (status IN ('pending', 'running') AND result IS NULL AND error IS NULL)
        OR (status = 'completed' AND result IS NOT NULL AND error IS NULL)
        OR (status = 'failed' AND result IS NULL AND error IS NOT NULL)
    )
);

COMMIT;
