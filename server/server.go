package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
	"unicode/utf8"
)

func checkNickname(nickname string) bool{
	if utf8.RuneCountInString(nickname)<5{
		return false
	}
	return true
}

func getRandomCoordinates() (x int, y int, s int){
	rand.Seed(time.Now().UnixNano())
	x=rand.Intn(700)+70
	y=rand.Intn(600)+70
	s=rand.Intn(14)+10
	return
}

func setCorrectForm(s string) string{
	ans := ""
	for _, num := range s{
		ans += string(num)
		if string(num) == "]"{
			return ans
		}
	}
	return ans
}



var queue chan []chan []byte
func init() {
	log.Println("Starting...")
}

type ProtectedBool struct{
	val bool
	sync.Mutex
}

func setProtectedBool(variable *ProtectedBool, val bool){
	variable.Lock()
	variable.val = val
	variable.Unlock()
}

func checkCoordinates(data []byte, x ,y ,s int) bool{
	var coordinates []float64
	fmt.Println(string(data))
	data = []byte(setCorrectForm(string(data)))
	fmt.Println(string(data))
	err := json.Unmarshal(data, &coordinates)

	if err!=nil{log.Println(err)}

	xg, yg := coordinates[0], coordinates[1]

	if (xg-float64(x))*(xg-float64(x))+(yg-float64(y))*(yg-float64(y))<=float64(s*s){
		return true
	}
	return false
}

// 0 - nickname, 1 - take_pistol, 2 - enemy_taked, 3 - shoot, 4 - result, 5 - stopReq, 6 - stopWait
func gameConnection(p1 []chan []byte, p2 []chan []byte){
	gameInfoDidntSended := true

	var (
		p1Nickname string
		p2Nickname string
		x, y, s int
		stop1 chan struct{} = make(chan struct{})
		stop2 chan struct{} = make(chan struct{})
	)



	p1Win := func () {
		go func () {
			select{
			case p1[4]<-[]byte("WIN"):
			}
			select{
			case p1[5]<-[]byte{}:
			}
		}()
	}

	p2Win := func () {
		go func (){
			select{
			case p2[4]<-[]byte("WIN"):
			case <-time.After(time.Second*5):
			}
			select{
			case p2[5]<-[]byte{}:
			case <-time.After(time.Second*5):
			}
		}()
	}

	for {
		select{
		case data:=<-p1[0]:
			p1Nickname=string(data)
		case data :=<-p2[0]:
			p2Nickname=string(data)
		case <-p1[1]:
			go func(){
				select{
				case p2[2]<-[]byte{}:
				case <-stop1:
				}
			}()
		case <-p2[1]:
			go func (){
				select{
				case p1[2]<-[]byte{}:
				case <-stop2:
				}
			}()
		case data:=<-p1[3]:
			if checkCoordinates(data, x, y, s){
				go p1Win()
				go func (){
					select{
					case p2[4]<-[]byte("LOSE"):
					case <-time.After(time.Second*5):
					}
					select{
					case p2[5]<-[]byte{}:
					case <-time.After(time.Second*5):
					}
				}()
				return
			}
		case data:=<-p2[3]:
			if checkCoordinates(data, x, y, s){
				go func () {
					select{
					case p1[4]<-[]byte("LOSE"):
					case <-time.After(time.Second*5):
					}
					select{
					case p1[5]<-[]byte{}:
					case <-time.After(time.Second*5):
					}
				}()
				go p2Win()
				return
			}
		case <-p1[6]:
			select{
			case stop1<-struct{}{}:
			default:
			}
			select{
			case stop2<-struct{}{}:
			default:
			}
			go p2Win()
			return

		case <-p2[6]:
			select{
			case stop1<-struct{}{}:
			default:
			}
			select{
			case stop2<-struct{}{}:
			default:
			}
			go p1Win()
			return
		case <-time.After(10*time.Second):

			go func (){
				select{
				case p2[4]<-[]byte("LOSE"):
				case <-time.After(time.Second*5):
				}
				select{
				case p2[5]<-[]byte{}:
				case <-time.After(time.Second*5):
				}
			}()
			go func () {
				select{
				case p1[4]<-[]byte("LOSE"):
				case <-time.After(time.Second*5):
				}
				select{
				case p1[5]<-[]byte{}:
				case <-time.After(time.Second*5):
				}
			}()
			return
		}

		if gameInfoDidntSended && p1Nickname != "" && p2Nickname != ""{
			x, y, s=getRandomCoordinates()
			packet1:=packGameHeader(p1Nickname, fmt.Sprintf("%v", x), fmt.Sprintf("%v", y), fmt.Sprintf("%v", s))
			packet2:=packGameHeader(p2Nickname, fmt.Sprintf("%v", x), fmt.Sprintf("%v", y), fmt.Sprintf("%v", s))
			p1[0]<-packet1
			p2[0]<-packet2
			gameInfoDidntSended = false
		}
	}
}

func packGameHeader(login string, x string, y string, s string) []byte{
	data, _ := json.Marshal([]string{login, x, y, s})
	return data
}


func clientConnection(c net.Conn) {
	var (
		nickname string
		// id int
		inGame ProtectedBool
		logined 	bool
		takedPistol bool
		stopSearching chan struct{} = make(chan struct{})
		stopChan chan []byte = make(chan []byte)
		nicknameChan chan []byte = make(chan []byte)
		takePistolChan chan []byte = make(chan []byte)
		enemyTakePistolChan chan []byte = make(chan []byte)
		shootChan chan []byte = make(chan []byte)
		resultChan chan []byte = make(chan []byte)
		stopChan2 chan []byte = make(chan []byte)
	)

	chanSlice := []chan []byte{nicknameChan, takePistolChan, enemyTakePistolChan, shootChan, resultChan, stopChan, stopChan2}

	resetGameSettings := func () {
		inGame.Lock()
		inGame.val = false
		inGame.Unlock()
		takedPistol = false
	}

	for {
		buffer := make([]byte, 10000)
		n, err := c.Read(buffer)
		if err!=nil {
			log.Println(err, c.RemoteAddr())
			c.Close()
			return
		}

		time.Sleep(time.Millisecond * 50)

		if buffer[0]==1 {
			received_nickname := buffer[1:n]
			if !checkNickname(string(received_nickname)){
				_, err:= c.Write([]byte("nickname must be at least 5 characters."))
				if err!=nil{log.Println(err)}
			}else{
				_, err:= c.Write([]byte("200"))
				if err!=nil{log.Println(err)}
				nickname = string(received_nickname)
				log.Println(nickname, "logged.")
				logined = true
			}
		}

		if buffer[0]==2 && logined{
			log.Println(c.RemoteAddr(), "searching game...")
			go  func ()  {
				select {
				case anotherPlayer:=<-queue:
					log.Println(c.RemoteAddr(), "found enemy. |r|")
					_, err := c.Write([]byte{200})
					if err!=nil{log.Println(err)}

					setProtectedBool(&inGame, true)
					// id = 1
					gameConnection(chanSlice, anotherPlayer)

				case queue<-chanSlice:
					log.Println(c.RemoteAddr(), "found enemy. |s|")
					_, err := c.Write([]byte{200})
					if err!=nil{log.Println(err)}

					setProtectedBool(&inGame, true)
					// id = 2

				case <- stopSearching:
				}
			}()
		}

		if buffer[0]==3 && logined && !inGame.val {
			stopSearching<-struct{}{}
			setProtectedBool(&inGame, false)
			log.Println(c.RemoteAddr(), "stoped searching.")
		}

		if buffer[0]==4 && logined && inGame.val { // game info
			nicknameChan<-buffer[1:n]

			data := <-nicknameChan

			_, err := c.Write(data)

			if err!=nil{log.Println(err)}
			go func () {
				for {
					select{
					case <-enemyTakePistolChan:
						_, err := c.Write([]byte("ENEMY_TAKED"))
						if err!=nil{log.Println(err)}
					case data:= <-resultChan:
						_, err:= c.Write(data)
						if err!=nil{log.Println(err)}
						resetGameSettings()
						break
					case <-stopChan:
					}
				}
			}()
		}
		
		if buffer[0]==5 && logined && inGame.val && !takedPistol{ // take pistol
			takePistolChan<-[]byte{}
			takedPistol = true
		}

		if buffer[0]==6 && logined && inGame.val && takedPistol{ // shoot
			shootChan<-buffer[1:n]
		}

		if buffer[0]==7 && logined && inGame.val {
			stopChan2<-[]byte{}
			stopChan<-[]byte{}
			resetGameSettings()
			_, err:=c.Write([]byte{200})
			if err!=nil{log.Println(err)}
		}
	}
}


func main(){
	if len(os.Args)<2{
		fmt.Println("port needed")
		os.Exit(1)
	}
	addr := fmt.Sprintf(":%v", os.Args[1])

	srv, err := net.Listen("tcp", addr)
	if err!=nil{log.Panic(err)}
	log.Println("Listening at port", addr)


	defer srv.Close()

	queue=make(chan []chan []byte)
	defer close(queue)

	for {
		conn, err := srv.Accept()
		if err!=nil {log.Panic(err)}
		conn.SetDeadline(time.Now().Add(360*time.Second))
		go clientConnection(conn)
	}
}

