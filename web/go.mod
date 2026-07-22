// This sentinel module keeps Go's ./... traversal out of the React workspace
// and its Node dependencies. The web application is built with npm/Vite.
module github.com/shikanon/cookies/web

go 1.26
