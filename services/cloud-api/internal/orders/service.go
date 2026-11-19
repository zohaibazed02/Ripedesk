package orders

type OrderMode string

const (
	ModeQuickSale OrderMode = "QUICK_SALE"
	ModeTakeaway  OrderMode = "TAKEAWAY"
	ModeDineIn    OrderMode = "DINE_IN"
	ModeDelivery  OrderMode = "DELIVERY"
	ModeQROrder   OrderMode = "QR_ORDER"
	ModeDriveThru OrderMode = "DRIVE_THRU"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Name() string {
	return "orders"
}
