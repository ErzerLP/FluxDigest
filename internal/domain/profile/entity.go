package profile

type Version struct {
	ID          string
	ProfileType string
	Name        string
	Version     int
	IsActive    bool
	PayloadJSON []byte
}
