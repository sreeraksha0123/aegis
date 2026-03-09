package ports

import "context"

type LimiterScore interface {
	Set(ctx context.Context, key string, value interface{}, ttlSeconds int) error
	Get(ctx context.Context, key string) interface{}
	Incr(ctx context.Context, key string) error
	Eval(ctx context.Context, script string, keys []string, args ...[]interface{}) (interface{}, error)
	ScriptLoad(ctx context.Context, script string) (string, error)
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...[]interface{}) (interface{}, error)
}
