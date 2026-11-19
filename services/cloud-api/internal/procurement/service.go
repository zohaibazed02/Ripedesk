package procurement

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Name() string {
	return "procurement"
}
