
package main

import ("fmt"
	"net"
	"flag"
	"bufio"
	"strconv"
	"sync"
	"os"
	"strings"
	"rand"
	"time")
var masterLoggingOn=flag.Bool("ml",false,"enable master logging?")
var loggingOn=flag.Bool("l",false,"enable logging?")
var usersId=map[string]int{}
var users =map[int]string{}
var chatrooms =map[int]string{}
var chatroomsId =map[string]int{}
func getRandomExistingUser() string {
	count:=len(users)
	return users[rand.Intn(count)]
}
func getRandomExistingChatroom() string {
	count:=len(chatrooms)
	if count==0 { return newChatroom() }
	return chatrooms[rand.Intn(count)]
}
func randomString() string {
	alphabets:="abcdefghijklmnopqrstuvwxyz"
	intArray:=rand.Perm(len(alphabets))
	rstring:=strings.Map(func (i int) int {return int(alphabets[intArray[i%26]])},
		alphabets)
	return rstring
}


var idChannel chan int

func randomUser() string {
	return randomString() }

func randomChatroom() string {
	return "#"+randomString()}

func randomMessage() string {
	return choose(func (str string) string {return str+"\r\n"},
		func (str string) string {return str+"\n"},
		func (str string) string {return str}).(func (string) string)(strings.Replace(strings.Repeat(randomString(),1+rand.Intn(1000)),"x"," ",-1))}

func toBaseN(id int,base int) map[int]int {
	m:=map[int]int{}
	i:=0
	for {
		v:=id%base
		id=id/base
		m[i]=v 
		if id==0 {break}
		i++		
	}
	return m
}

func idToString(id int) string {
	alphabets:="abcdefghijklmnopqrstuvwxyz"
	idBase27:=toBaseN(id,26)

	byteArray:=make([]byte,len(idBase27))
	for i:=0;i<len(idBase27);i++ {
		byteArray[i]=alphabets[idBase27[i]]}
	return string(byteArray)
}

func constantFunc(a string) (func () string) {
	return func () string {return a} }

func choose(choices ...interface{}) interface{} {
	return choices[rand.Intn(len(choices))]
}


func chooseRandomValidCommand() string {
	commands :=[...]string{"LOGIN","LOGOUT","PART","JOIN","MSG"}
	return commands[rand.Intn(len(commands))]
}

func selectRandomKeys(m map[string]int , p float32) (ret map[string]bool) {
	n:=len(m)
	threshold:=int(p*float32(n))
	ret=map[string]bool{}
	for str,_:=range m {
		if rand.Intn(n)<=threshold {
			ret[str]=true }}
	return ret	
}

func newUser() string {
	curId:=len(users)
	nuser:=idToString(curId)
	users[curId]=nuser;
	usersId[nuser]=curId;
	return nuser
}

func newChatroom() string {
	curId:=len(chatrooms)
	nchatroom:="#"+idToString(curId)
	chatroomsId[nchatroom]=curId
	chatrooms[curId]=nchatroom
	return nchatroom
}

func errorString(info string,err os.Error) string {
	return fmt.Sprintf("%s %s temporary : _ can be timedout : _",info,err.String());
}

func printError(info string,err os.Error) {
	println(info,err.String());
}


// read from connection and return true if ok
func Read(con *net.Conn) (reader chan string) {
	reader=make(chan string)
//	var buf = make ([]byte,4048);
	networkReader:=bufio.NewReader(*con)
	go func() {
		for {
			str, err := networkReader.ReadString('\n');
			if err!=nil {
/*				if !closed(reader) {
					reader<-errorString("netConn Read : ",err)
				}*/
				if err==os.EOF { close(reader) }
			} else {
				reader<-str
	}}} ()
	return reader
}

// clientsender(): read from stdin and send it via network

func dbg(str string) string {
	println("debug : ",str)
	return str
}


func idGenerator() {
	i:=0
	idChannel=make(chan int)
	go func () {
		for { 
			i++
			idChannel<-i }} ()
}
var masterLoggerChannel chan string=make(chan string)
var masterLoggerConfirmationChannel chan bool=make(chan bool)
func masterLogger(fname string) {
	if *masterLoggingOn {
		println("master logging turned on")
		fd,err:=os.OpenFile(fname,os.O_WRONLY|os.O_CREATE,0644)
		if err!=nil { 
			println(err.String()) 
		} else {
			go func() {
				for str:=range masterLoggerChannel {
					if !strings.HasPrefix(str,"user") {
						panic("something wrong") 
					}
					fmt.Fprintf(fd,str)
					masterLoggerConfirmationChannel<-true
				}
				fd.Close()
			}()
}}}

func createClient(wg *sync.WaitGroup) {
	destination := "127.0.0.1:9988";
	clientId:=<-idChannel
	loggerChannel:=make(chan string)
	loggerConfirmationChannel:=make(chan bool)
	go func() {
		wg.Add(1)
		defer wg.Done()
		var fd *os.File
		
		var err os.Error
		if *loggingOn {
			file:="user"+strconv.Itob(clientId,10)+".log"
			fd,err=os.OpenFile(file, os.O_WRONLY | os.O_CREATE | os.O_TRUNC | os.O_SYNC,0644)
			if err != nil {
				println(fmt.Sprintf("%v : ",file)+errorString("",err))
				return
			}
			defer fd.Close()
		}
		for str:=range loggerChannel {
			if *loggingOn {
				fmt.Fprintf(fd,str)
			} else if *masterLoggingOn {
				masterMessage:="user"+strconv.Itob(clientId,10)+" : "+str
				masterLoggerChannel<-masterMessage
				<-masterLoggerConfirmationChannel
			}
			loggerConfirmationChannel<-true
		}
		
	} ()
	
	cn, netErr := net.Dial("tcp", destination);
	if netErr!=nil {
		loggerChannel<-fmt.Sprintf("connection error for client %d : ",clientId)+errorString("",netErr)
		<-loggerConfirmationChannel
		return
	}
	
	
	clientsender:= func () {
		wg.Add(1)
		defer wg.Done()
		ns:=int64(rand.Intn(100000))
		triggerChannel:=time.Tick(ns)
		_,nCmdChannel:=randomValidFullCommand()
		readerChannel:=Read(&cn)
		for {
			<-triggerChannel
			select {
			case reply:=<-readerChannel:
				loggerChannel<-reply
				<-loggerConfirmationChannel
			case ncmd:=<-nCmdChannel:
				loggerChannel<-ncmd
				<-loggerConfirmationChannel
				cn.Write([]byte(ncmd))
			}
			if closed(nCmdChannel) || closed(readerChannel) { 
				close(loggerChannel)
				close(loggerConfirmationChannel)
				break 
	}}}
	go clientsender();
}

func randomMapEntry(m map[int]string) (int,string) {
	n:=rand.Intn(len(m))
	count:=0
	for key,val:=range m {
		if count==n {
			return key,val}
		count++
	}
	return -1,""
}

func randomValidFullCommand() (myName string,fullCmdChannel chan string) {
	fullCmdChannel=make(chan string)
	mychatroomsId:=map[string]int{}
	mychatrooms:=map[int]string{}
	curChatroomId:=0
	myName=newUser()
	go func () {
		fullCmdChannel<-"LOGIN "+myName+"\r\n"
		joinChatRoom := func () string { 
			var mynewchatroom string;
			for { 
				mynewchatroom=choose(newChatroom,getRandomExistingChatroom).(func () string)()
				_,present:=mychatroomsId[mynewchatroom]
				if !present {
					mychatroomsId[mynewchatroom]=curChatroomId
					mychatrooms[curChatroomId]=mynewchatroom 
					break}}
			return "JOIN "+mynewchatroom }
		partChatRoom := func () string {
			if len(mychatrooms)==0 {joinChatRoom()}
			id,partingRoom:=randomMapEntry(mychatrooms)
			mychatrooms[id]="",false
			mychatroomsId[partingRoom]=-1,false
			return "PART "+partingRoom }
		messageMyChatRoom := func () string {
			if len(mychatrooms)==0 {joinChatRoom()}
			_,messagingRoom:=randomMapEntry(mychatrooms)
			return "MSG "+messagingRoom+" "+randomMessage() }
		messageUser := func () string {
			_,messagingUser:=randomMapEntry(users)
			return "MSG "+messagingUser+" "+randomMessage() }
		nCommands:=rand.Intn(1000)
		for i:=0;i<nCommands;i++ {
			ncmd:=choose(joinChatRoom,partChatRoom,messageUser,messageMyChatRoom).(func () string)()+"\r\n"
			//println("emitting ",i,"th command of ",nCommands)
			fullCmdChannel<-ncmd
			//println("current command : ",ncmd)
		}
		
		fullCmdChannel<-"LOGOUT\r\n"
		close(fullCmdChannel)
	} ()
	return myName,fullCmdChannel
}

func main(){
	nUsers:=flag.Int("u",1,"number of users")
	nChatrooms:=flag.Int("r",0,"number of chatrooms")
	flag.Parse()
	println(" nusers : ",*nUsers)
	println(" rooms  : ",*nChatrooms)
	println(" loggingOn : ",*loggingOn)
	println(" masterLoggingOn : ",*masterLoggingOn)
	masterLogger("badClient.master.log")
	var wg sync.WaitGroup
	idGenerator()
	for i:=0;i<*nChatrooms;i++ {newChatroom()}
	for i:=0;i<*nUsers;i++ {  createClient(&wg)}
	wg.Wait()
	close(masterLoggerChannel)
}