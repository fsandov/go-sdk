package web

// RequestIDContextKey is the context key used to propagate request IDs
// from server middleware to outbound client middleware via context.Context.
//
// NOTE: The concrete key type lives in pkg/client (client.RequestIDContextKey)
// because pkg/web already imports pkg/client — defining the key there avoids
// a circular import while keeping the same compile-time type-safety guarantee.
// This file exists as the canonical documentation point for the end-to-end
// request-ID propagation contract.
//
// Flow:
//  1. Inbound HTTP request arrives at pkg/web.RequestIDMiddleware.
//  2. The request ID (from X-Request-ID header or a new UUID) is stored both
//     in the gin context ("request_id" key) and in the stdlib context via
//     client.RequestIDContextKey{}.
//  3. When the service makes an outbound call, pkg/client.RequestIDMiddleware
//     reads client.RequestIDContextKey{} from the request context and forwards
//     the same ID in the X-Request-ID header — creating a traceable chain
//     across service boundaries.
