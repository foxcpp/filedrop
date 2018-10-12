package filedrop

import "net/http"

// filedrop server structure, implements http.Handler.
type Server struct {
	DB *db
}

func New(conf Config) (*Server, error) {
	s := new(Server)
	var err error

	s.DB, err = openDB(conf.DB.Driver, conf.DB.DSN)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}

func (s *Server) Close() error {
	return s.DB.Close()
}
