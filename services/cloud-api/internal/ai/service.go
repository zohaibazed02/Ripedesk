package ai

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Name() string {
	return "ai"
}
