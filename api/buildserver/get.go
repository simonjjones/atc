package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc/api/present"
)

func (s *Server) GetBuild(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dbBuild, err := s.db.GetBuild(buildID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	build := present.Build(dbBuild)

	json.NewEncoder(w).Encode(build)
}
