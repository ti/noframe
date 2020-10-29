# usage

## steps1

### 1. write your proto to define the apis

exmple `main.proto`

### 2. build the server interface code

`make`

### 3. write your server code to implement the proto

exmple `main.go`


### 4. run

`go run main.go`

### 5. Debug

`curl http://127.0.0.1:8080/v1/greeter/hello/12`
