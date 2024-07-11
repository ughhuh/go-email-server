module github.com/ughhuh/go-email-server

go 1.22.4

require (
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	github.com/phires/go-guerrilla v1.6.6
)

require (
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/yuin/goldmark v1.4.13 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/tools v0.23.0 // indirect
)

replace github.com/ughhuh/go-email-server/backend v0.0.0 => ../backend
