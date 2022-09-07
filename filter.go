package apmbeego // import "github.com/fandi-abdillah/apmbeego"

import (
	"context"
	"net/http"

	beego "github.com/beego/beego/v2/server/web"
	beegocontext "github.com/beego/beego/v2/server/web/context"

	"go.elastic.co/apm/module/apmhttp/v2"
	"go.elastic.co/apm/v2"
)

type beegoFilterStateKey struct{}

type beegoFilterState struct {
	context *beegocontext.Context
}

func init() {
	AddFilters(beego.BeeApp.Handlers)
	WrapRecoverFunc(beego.BConfig)
}

// Middleware returns a beego.MiddleWare that traces requests and reports panics to Elastic APM.
func Middleware(o ...Option) func(http.Handler) http.Handler {
	opts := options{
		tracer: apm.DefaultTracer(),
	}
	for _, o := range o {
		o(&opts)
	}
	return func(h http.Handler) http.Handler {
		return apmhttp.Wrap(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			tx := apm.TransactionFromContext(req.Context())
			if tx != nil {
				state := &beegoFilterState{}
				defer setTransactionContext(tx, state)
				ctx := context.WithValue(req.Context(), beegoFilterStateKey{}, state)
				req = apmhttp.RequestWithContext(ctx, req)
			}
			h.ServeHTTP(w, req)
		}), apmhttp.WithTracer(opts.tracer), apmhttp.WithServerRequestName(apmhttp.UnknownRouteRequestName))
	}
}

// AddFilters adds required filters to handlers.
//
// This is called automatically for the default app (beego.BeeApp),
// so if you beego.Router, beego.RunWithMiddleware, etc., then you
// do not need to call AddFilters.
func AddFilters(handlers *beego.ControllerRegister) {
	handlers.InsertFilter("*", beego.BeforeStatic, beforeStatic)
}

// WrapRecoverFunc updates config's RecoverFunc so that panics will be reported to Elastic APM
// for traced requests. For non-traced requests, the original RecoverFunc will be called.
//
// WrapRecoverFunc is called automatically for the global config, beego.BConfig.
func WrapRecoverFunc(config *beego.Config) {
	orig := config.RecoverFunc
	config.RecoverFunc = func(context *beegocontext.Context, config *beego.Config) {
		if tx := apm.TransactionFromContext(context.Request.Context()); tx == nil {
			orig(context, config)
		}
	}
}

func beforeStatic(context *beegocontext.Context) {
	state, ok := context.Request.Context().Value(beegoFilterStateKey{}).(*beegoFilterState)
	if ok {
		state.context = context
	}
}

func setTransactionContext(tx *apm.Transaction, state *beegoFilterState) {
	tx.Context.SetFramework("beego", "2.0.2")
	if state.context != nil {
		if route, ok := state.context.Input.GetData("RouterPattern").(string); ok {
			tx.Name = state.context.Request.Method + " " + route
		}
	}
}

type options struct {
	tracer *apm.Tracer
}

// Option sets options for tracing.
type Option func(*options)

// WithTracer returns an Option which sets t as the tracer to use for tracing server requests.
func WithTracer(t *apm.Tracer) Option {
	if t == nil {
		panic("t == nil")
	}
	return func(o *options) {
		o.tracer = t
	}
}
