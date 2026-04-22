package middleware

import "github.com/valyala/fasthttp"

func CORSMiddleware(nextFunction fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(c *fasthttp.RequestCtx) {
		c.Response.Header.Set("Access-Control-Allow-Origin", "*")
		c.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Authorization")
		c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if c.IsOptions() {
			c.SetStatusCode(fasthttp.StatusNoContent)
			return
		}
		nextFunction(c)
	}
}
