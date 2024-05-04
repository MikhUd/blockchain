package db

type Connector interface {
	Connect() error
}
