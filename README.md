# peripheral_backend-in_go

來做HA高可用性

# proto generate

protoc --proto_path=./proto \
 --go_out=./protoGen --go_opt=paths=source_relative \
 --go-grpc_out=./protoGen --go-grpc_opt=paths=source_relative \
 ha.proto

protoc --proto_path=./proto \
 --go_out=./protoGen --go_opt=paths=source_relative \
 --go-grpc_out=./protoGen --go-grpc_opt=paths=source_relative \
 server.proto

# schema

mysqldump -u root -p --no-data corning_v2 > schema.sql
