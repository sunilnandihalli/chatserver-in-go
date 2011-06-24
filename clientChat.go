
package main

import ("fmt";
	"net";
	"log";
	"os";
//	"bytes";
	"bufio";
//	"strings";
	"time";
	"flag";)

var running bool;  // global variable if client is running

var debug = flag.Bool("d", false, "set the debug modus( print informations )")

// func Log(v ...): loging. give log information if debug is true
func Log(v ...interface {}) {
	if *debug == true {
		ret := fmt.Sprint(v...);
		log.Printf("CLIENT: %s", ret);
	}
}

// func test(): testing for error
func test(err os.Error, mesg string) {
	if err!=nil {
		log.Panicf("CLIENT: ERROR: %s", mesg);
		os.Exit(-1);
	} else
		Log("Ok: ", mesg);
}

// read from connection and return true if ok
func Read(con *net.Conn) string{
	var buf = make ([]byte,4048);
	_, err := (*con).Read(buf);
	if err!=nil {
		(*con).Close();
		running=false;
		return "Error in reading!";
	}
	str := string(buf);
	fmt.Println();
	return string(str);
}

func printError(info string,err os.Error) {
	println(info,err.String());
}

// clientsender(): read from stdin and send it via network
func clientsender(cn *net.Conn) {
	reader := bufio.NewReader(os.Stdin);
	for {
		fmt.Print("you> ");
		input, err := reader.ReadString('\n');
		//(*cn).Write([]byte(strings.Replace(input,"\n","\r\n",1)));
		if err!= nil {
			printError("input error : ",err) }
		(*cn).Write([]byte(input));
		
	}
}

// clientreceiver(): wait for input from network and print it out
func clientreceiver(cn *net.Conn) {
	for running {
		fmt.Println(Read(cn));
		fmt.Print("you> ");
	}
}

func main() {
	flag.Parse();
	running = true;
	Log("main(): start ");
	
	// connect
	destination := "127.0.0.1:9988";
	Log("main(): connecto to ", destination);
	cn, err := net.Dial("tcp",  destination);
	test(err, "dialing");
	defer cn.Close();
	Log("main(): connected ");
	
	
	// start receiver and sender
	Log("main(): start receiver");
	go clientreceiver(&cn);
	Log("main(): start sender");
	go clientsender(&cn);
	
	// wait for quiting (/quit). run until running is true
	for ;running; {
		time.Sleep(1*1e9);
	}
	Log("main(): stoped");
}