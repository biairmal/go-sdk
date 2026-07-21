package response

// Success carries HTTP status and data for a successful response.
// The handler adapter uses HTTPStatusCode to set the response status
// and Data for the response body.
type Success struct {
	HTTPStatusCode int
	Data           any
}
