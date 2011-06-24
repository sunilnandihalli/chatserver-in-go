
all: server client badclient

server: chatServer.go
	8g chatServer.go; 8l -o server chatServer.8 ;

client: clientChat.go
	8g clientChat.go; 8l -o client clientChat.8;

badclient: badClient.go
	8g badClient.go; 8l -o badclient badClient.8;

clean: 
	\rm -vf *.8 client server badclient
