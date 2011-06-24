package main

import ("net"
	"fmt"
	"sync"
	"flag"
	"os"
	"bufio"
	"strings")

var (MaxMessageLength = flag.Int("-m",4000,"maximum message length"))
func errorString(info string,err os.Error) string {
	return fmt.Sprintf("%s %s temporary : _ can be timedout : _",info,err.String());
}

func printError(info string,err os.Error) {
	println(info,err.String());
}


type Command int
const (	INVALID Command = iota
	LOGIN
	ABORT
	GET_OR_CREATE_CHATROOM
	DESTROY_CHATROOM
	JOIN
	PART
	MSG
	PRINT
	PRINTERROR
	GOTROOMMSG
	GOTUSERMSG
	LOGOUT
	IGNORE
	GET_USER_CHANNEL
	PUT_USER_CHANNEL)

const ( USER_NOT_FOUND = "user not found"
	VERY_LONG_MESSAGE = "too long a message"
	USER_ALREADY_PRESENT = "user already present"
	USER_NOT_MEMBER_OF_CHATROOM = "you are not a member of this chat room"
	CHATROOM_NOT_PRESENT = "the requested chatroom does not exist"
	INVALID_USER_NAME = "the user name consist of only alphanumeric characters"
	INVALID_CHATROOM_NAME = "the chatroom name should always start with an # followed by alphanumeric characters"
	SERVER_BUSY = "too many users are already connected to the server... Please wait"
	OK = "OK")

var (manageUsersChannel = make(chan *statement)
	manageChatroomsChannel = make(chan *statement)
	stringToCommand = map[string]Command{ "LOGIN":LOGIN,"JOIN":JOIN,"PART":PART,
		"MSG":MSG,"GOTROOMMSG":GOTROOMMSG,"GOTUSERMSG":GOTUSERMSG,"LOGOUT":LOGOUT,"IGNORE":IGNORE}
	commandToString = map[Command]string{ LOGIN:"LOGIN",JOIN:"JOIN",PART:"PART",
	MSG:"MSG",GOTROOMMSG:"GOTROOMMSG",GOTUSERMSG:"GOTUSERMSG",LOGOUT:"LOGOUT",IGNORE:"IGNORE"}
	allowLoginsFromSameMachineToReuseSameConnectionAfterLogout = true)

type statement struct {
	cmd Command
	name string
	msg string
	curUserChannel chan* statement
	responseReady chan bool
	reply interface{}}

func isValidUserName(str string) bool {
	return true
}

func isValidChatroomName(str string) bool {
	return true
}

func makeStatement(inp string) (st *statement,err string) {
	out := strings.Fields(inp)
	var curcmd Command
	var ch chan bool
	var curname string
	var curmsg string
	if(len(out)>0) {
		curcmd = stringToCommand[out[0]]
		ch = make(chan bool)
		switch curcmd {
		case LOGIN : 
			curname=out[1]
			switch {
			case !isValidUserName(curname) : return nil,curname+" is not a valid username "
			case len(out)>2 : return nil,"a login statement should have exactly one argument "
			}
		case PART : fallthrough
		case JOIN : 
			curname=out[1]
			switch {
			case !isValidChatroomName(curname) : return nil,curname+" is not a valid chatroom name "
			case len(out)>2 : return nil,commandToString[curcmd]+" should accompanied by only a chatroom name"
			}			
		case MSG :
			curname=out[1]
			if !isValidChatroomName(curname) && !isValidUserName(curname) {
				return nil,curname+" is not a valid chatroom or user name "
			}
		case LOGOUT :
			if len(out)>1 {
				return nil,"LOGOUT should not have any accompanying arguments"
			}
		}
		if(len(out)>1) {
			curname = out[1]}
		if(len(out)>2) {
			curmsg=strings.Join(out[2:]," ")}}
	switch{
	case len(out)==0:
		st = &statement{cmd:IGNORE}
	case  len(out)==1:	
		st =  &statement{cmd:curcmd,responseReady:ch}
	case len(out)==2: 
		st =  &statement{cmd:curcmd, name:curname,responseReady:ch}
	case len(out)>2:
		st =  &statement{cmd:curcmd, name:curname, msg:curmsg,responseReady:ch}}
	return st,""
}
	
func manageUsers(){
	userNamesToUserChannels := map[string]chan *statement{}
	for {
		st:= <-manageUsersChannel
		switch st.cmd {
		case LOGIN:
			_,present:=userNamesToUserChannels[st.name]
			if(!present){
				v:=make(chan *statement)
				st.reply=v
				userNamesToUserChannels[st.name]=v
			} else {
				st.reply = "user name "+st.name+" already exists"}
			st.responseReady<-true
		case LOGOUT:
			curUserChannel,present:=userNamesToUserChannels[st.name]
			killSenderGoroutineStatement:=&statement{cmd:LOGOUT,responseReady:make(chan bool)}
			if(present) {
				curUserChannel<-killSenderGoroutineStatement
				<-killSenderGoroutineStatement.responseReady
				close(curUserChannel)
				userNamesToUserChannels[st.name]=nil,false} else {
				panic("manageUsers : trying to logout of a non-existent user "+st.name) }
			st.responseReady<-true
		case GET_USER_CHANNEL:
			ch,present := userNamesToUserChannels[st.name]
			if(present) {
				st.reply = ch } else {
				st.reply = USER_NOT_FOUND }			
			st.responseReady<-true
}}}

func manageChatRooms() {
	chatroomNamesToChatroomChannels:= map[string]chan *statement{}
	for {
		st:=<-manageChatroomsChannel
		switch st.cmd {
		case GET_OR_CREATE_CHATROOM:
			chatroomChannel,chatroomExists:=chatroomNamesToChatroomChannels[st.name]
			if(!strings.HasPrefix(st.name,"#")) {
				st.reply="Please use a chatroom name starting with #"} else {
				if(!chatroomExists) { 
					chatroomChannel = makeChatRoom(st.name)
					chatroomNamesToChatroomChannels[st.name]=chatroomChannel,true
				}
				st.reply=chatroomChannel}
			st.responseReady<-true
		case DESTROY_CHATROOM: //only for self destruction when everybody vacates the chatroom
			_,chatroomExists:=chatroomNamesToChatroomChannels[st.name]
			if(!chatroomExists) {
				st.reply="trying to kill the chatroom "+st.name+" that does not exist"
				st.responseReady<-true
			} else {
				chatroomNamesToChatroomChannels[st.name]=nil,false
				st.reply=OK
				st.responseReady<-true 
}}}}

func makeChatRoom(name string) (chan * statement) { // it is simply a one to many connection with its own name and maintains a membership ... 
	curChatroomChannel:=make(chan *statement)
	chatroom:=func () {
		curChatroomName :=name
		curChatroomMemberChannels:=map[string]chan*statement{}
		for {
			chatroomStatement := <-curChatroomChannel
			switch chatroomStatement.cmd {
			case JOIN:
				if(strings.HasPrefix(chatroomStatement.name,"#")) { panic("incorrect")}
				curChatroomMemberChannels[chatroomStatement.name]=chatroomStatement.curUserChannel,true
				chatroomStatement.reply=curChatroomChannel
				chatroomStatement.responseReady<-true
			case PART:
				_,userMemberOf:=curChatroomMemberChannels[chatroomStatement.name]
				if userMemberOf {
					curChatroomMemberChannels[chatroomStatement.name]=nil,false
					if (len(curChatroomMemberChannels)==0) {
						killChatroomStatement:=&statement{cmd:DESTROY_CHATROOM,name:curChatroomName,responseReady:make(chan bool)}
						manageChatroomsChannel<-killChatroomStatement
						<-killChatroomStatement.responseReady
					}
					chatroomStatement.reply=OK
					chatroomStatement.responseReady<-true 
				} else {
					panic("user "+chatroomStatement.name+" is trying to leave a chatroom that he is not member of ..")
				}
			case PRINT:
				broadcastStatement:=&statement{cmd:PRINT,responseReady:make(chan bool),msg:chatroomStatement.msg}
				var wg sync.WaitGroup;
				f:=func (ch chan * statement) {
					wg.Add(1)
					defer wg.Done()
					ch<-broadcastStatement
					<-broadcastStatement.responseReady
					}
				for _,channel:=range curChatroomMemberChannels {
					go f(channel)}
				wg.Wait()
				chatroomStatement.reply=OK
				chatroomStatement.responseReady<-true
	}}}
	go chatroom()
	return curChatroomChannel
}
	
func makeUser(conn *net.Conn) {
	var myChannel chan *statement;
	var loginSuccess bool;
	var myName string;
	networkReader := bufio.NewReader(*conn)
	println("size of buffer : ",networkReader.Buffered())
	userSender := func () {
		for {
			st:=<-myChannel
			switch st.cmd{
			case PRINT:
				(*conn).Write([]byte(st.msg+"\n"))
				st.reply=OK
				st.responseReady<-true
			case PRINTERROR:
				(*conn).Write([]byte("ERROR : "+st.msg+"\n"))
				st.reply=OK
				st.responseReady<-true
			case LOGOUT:
				st.responseReady<-true
				return
			default:
				println("userSender : no statement was executed")
	}}}
	userReciever := func () {
		myChatrooms:=map[string]chan*statement{}
		for {
			line,err:=networkReader.ReadString('\n')
			if(err!=nil) {
				if err==bufio.ErrBufferFull {
					errorStatement:=&statement{cmd:PRINTERROR,msg:"message too long ignoring this message",responseReady:make(chan bool)}
					for {
						_,err:=networkReader.ReadString('\n')
						if err!=nil || err!=bufio.ErrBufferFull { break }
					}
					myChannel<-errorStatement
					<-errorStatement.responseReady
					continue
				} else {
					line="LOGOUT\r\n"
					println(err.String())
				}
			}
			curStatement,statementErr:=makeStatement(line)
			if curStatement!=nil {
			switch curStatement.cmd {
				case  MSG:
					if(strings.HasPrefix(curStatement.name,"#")) {
						func () {
							defer func () {
								if r:=recover(); r!= nil {
									errorMessage:="unable to message the chatroom "+curStatement.name
									confirmationStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:errorMessage}
									myChannel<-confirmationStatement
									<-confirmationStatement.responseReady
							}} ()
							chatroomChannel,memberOf:=myChatrooms[curStatement.name]
							var confirmationStatement *statement
							if memberOf { 
								brdcstmsg:="GOTROOMMSG "+myName+" "+curStatement.name+" "+curStatement.msg
								broadcastStatement:=&statement{cmd:PRINT,responseReady:make(chan bool),msg:brdcstmsg}
								chatroomChannel<-broadcastStatement 
								<-broadcastStatement.responseReady
								confirmationStatement=&statement{cmd:PRINT,responseReady:make(chan bool),msg:OK} } else {
								errorMessage:="Please join the group "+curStatement.name+" before messaging to it"
								confirmationStatement=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:errorMessage}}
							myChannel<-confirmationStatement
							<-confirmationStatement.responseReady 
						} ()
					} else {
						func () {
							defer func () {
								if r:=recover();r!=nil {
									errorMessage:="unable to message the user "+curStatement.name
									confirmationStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:errorMessage}
									myChannel<-confirmationStatement
									<-confirmationStatement.responseReady
							}} ()
						getUserChannelStatement := &statement{cmd:GET_USER_CHANNEL,name:curStatement.name,responseReady:make(chan bool)}
						manageUsersChannel<-getUserChannelStatement
						<-getUserChannelStatement.responseReady
						user_channel,isChannel := getUserChannelStatement.reply.(chan *statement)
						var confirmationStatement *statement
						if(isChannel) {
							curmsg:="GOTUSERMSG "+myName+" "+curStatement.msg
							messagePrintStatement := &statement{cmd:PRINT,msg:curmsg,responseReady:make(chan bool)}
							user_channel<-messagePrintStatement
							<-messagePrintStatement.responseReady
							confirmationStatement = &statement{cmd:PRINT,msg:"OK",responseReady:make(chan bool)} } else {
							confirmationStatement = &statement{cmd:PRINTERROR,msg:getUserChannelStatement.reply.(string),responseReady:make(chan bool)}
						}
						myChannel<-confirmationStatement
						<-confirmationStatement.responseReady
						} ()
					}
				case  JOIN:
					func () {
						defer func () {
							if r:=recover();r!=nil {
								errorMessage:="unable to join the chatroom "+curStatement.name
								confirmationStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:errorMessage}
								myChannel<-confirmationStatement
								<-confirmationStatement.responseReady
						}} ()
					getOrCreateChatroomStatement:=&statement{cmd:GET_OR_CREATE_CHATROOM,name:curStatement.name,responseReady:make(chan bool)}
					manageChatroomsChannel<-getOrCreateChatroomStatement
					<-getOrCreateChatroomStatement.responseReady
					chatroomChannel,getOrCreateChatroomSuccessfull:=getOrCreateChatroomStatement.reply.(chan*statement)
					var confirmationStatement*statement;
					if(getOrCreateChatroomSuccessfull) {
						joinChatroomStatement:=&statement{cmd:JOIN,name:myName,curUserChannel:myChannel,responseReady:make(chan bool)}
						chatroomChannel<-joinChatroomStatement
						<-joinChatroomStatement.responseReady
						confirmationStatement=&statement{cmd:PRINT,msg:OK,responseReady:make(chan bool)}
						myChatrooms[curStatement.name]=chatroomChannel } else {
						confirmationStatement=&statement{cmd:PRINTERROR,msg:getOrCreateChatroomStatement.reply.(string),responseReady:make(chan bool)} }
					myChannel<-confirmationStatement
					<-confirmationStatement.responseReady
					} ()
				case  PART:
					func () {
						defer func () {
							if r:=recover();r!=nil {
								errorMessage:="unable to leave the chatroom "+curStatement.name
								confirmationStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:errorMessage}
								myChannel<-confirmationStatement
								<-confirmationStatement.responseReady
						}} ()
					chatroomChannel,memberOf:=myChatrooms[curStatement.name]
					var confirmationStatement*statement
					if memberOf { 
						chatroomStatement:=&statement{cmd:PART,name:myName,curUserChannel:myChannel,responseReady:make(chan bool)}
						chatroomChannel<-chatroomStatement
						<-chatroomStatement.responseReady
						myChatrooms[curStatement.name]=nil,false
						confirmationStatement=&statement{cmd:PRINT,responseReady:make(chan bool),msg:OK} } else {
						confirmationStatement=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:USER_NOT_MEMBER_OF_CHATROOM+" "+curStatement.name}}
					myChannel<-confirmationStatement
					<-confirmationStatement.responseReady
					} ()
				case  LOGIN:
					errorStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:"already logged in as "+myName}
					myChannel<-errorStatement
					<-errorStatement.responseReady					
				case  LOGOUT:
					for _,chatroomChannel:= range myChatrooms {
						chatroomStatement:=&statement{cmd:PART,name:myName,curUserChannel:myChannel,responseReady:make(chan bool)}
						chatroomChannel<-chatroomStatement
						<-chatroomStatement.responseReady }
					manageUsersStatement:=&statement{cmd:LOGOUT,name:myName,responseReady:make(chan bool)}
					manageUsersChannel<-manageUsersStatement
					<-manageUsersStatement.responseReady
					(*conn).Close()
					return
				case IGNORE:
				case INVALID:
					confirmationStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:"invalid command \r\n"}
					myChannel<-confirmationStatement
					<-confirmationStatement.responseReady
				default:
					println("user reciever : no statement was executed ..." )
			}} else {
				errorStatement:=&statement{cmd:PRINTERROR,responseReady:make(chan bool),msg:statementErr}
				myChannel<-errorStatement
				<-errorStatement.responseReady
	}}}
	for{
		line,err:=networkReader.ReadString('\n')
		if(err!=nil) {
			println(err.String())
			(*conn).Close()
			return
		}
		st,statementErr := makeStatement(line)
		if (st!=nil) {
			if (st.cmd == LOGIN) {
				manageUsersChannel<- st
				<-st.responseReady
				myChannel,loginSuccess = st.reply.(chan *statement)
				if(loginSuccess) {
					myName = st.name
					go userSender()
					go userReciever()
					nst:=&statement{cmd:PRINT,msg:OK,responseReady:make(chan bool)}
					myChannel<-nst
					<-nst.responseReady
					break
				} else {
					errorMessage,isAnErrorMessage := st.reply.(string)
					if(isAnErrorMessage) {
						(*conn).Write([]byte(errorMessage+"\r\n")) } else {
						(*conn).Write([]byte("unknown error occurred during login, please retry\r\n"))}}} else {
				(*conn).Write([]byte("ERROR : please login before issuing any other command\r\n"))}} else {
			(*conn).Write([]byte("ERROR : "+statementErr+"\n"))
}}}
	
func main() {
	port:=flag.Int("p",9988,"port number to run the server on")
	flag.Parse()	
	netlisten, netlistenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d",*port));
	if netlistenErr!=nil {
		printError("netlisten error : ", netlistenErr)
			return
	}
	go manageUsers()
	go manageChatRooms()
	for {
		conn, connAcceptErr := netlisten.Accept();
			if connAcceptErr!=nil {
			println(errorString("connection accept : ",connAcceptErr))
		} else {
			go makeUser(&conn) 
}}}


