export DB_USER_NAME="psql_user"
export DB_PASSWORD="pasword"
export DB_HOST="localhost"
export DB_PORT="5432"
export DB_NAME="benchmark"
export DB_PARAMS="sslmode=disable"

go test -bench=. -benchtime=1000x -timeout 60m
