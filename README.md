# peripheral_backend-in_go

來做HA高可用性
因為兩邊的程式是一樣的
所以會同時有server client的角色在此專案
當連到另外HA時
server負責接收資料
client負責發送資料
HA_1 CLIENT -> HA_2 SERVER -> HA_2 CLIENT -> HA_1 SERVER

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
