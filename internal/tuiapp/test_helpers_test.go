package tuiapp

type fakeJobStore struct {
	loaded []Job
	saved  []Job
	err    error
}

func (s *fakeJobStore) LoadJobs() ([]Job, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]Job(nil), s.loaded...), nil
}

func (s *fakeJobStore) SaveJobs(jobs []Job) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append([]Job(nil), jobs...)
	return nil
}
