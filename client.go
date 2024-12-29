package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

func clearWin(win *gtk.Window) {
	win.GetChildren().Foreach(func(child any) {
		tmp, _ := child.(*gtk.Widget).Cast()
		win.Remove(tmp)
	})
}
func unPackGameHeader(data []byte) (login string, x string, y string, s string){
	var a []string
	json.Unmarshal(data, &a)
	login, x, y, s = a[0], a[1], a[2], a[3]
	return
}

func readStop(c net.Conn) {
	c.SetReadDeadline(time.Now())
	time.Sleep(10*time.Millisecond)	
	c.SetReadDeadline(time.Time{})
}

type ProtectedUint struct{
	val glib.SignalHandle
	sync.Mutex
}

var SignalHandler ProtectedUint

func drawTarget(da *gtk.DrawingArea, xh string, yh string, sh string) {
	
	x, _:=strconv.Atoi(xh)
	y, _:=strconv.Atoi(yh)
	s, _:=strconv.Atoi(sh)


	da.Connect("draw", func(da *gtk.DrawingArea, cr *cairo.Context) {
		cr.SetSourceRGB(255, 0, 0)
		cr.Arc(float64(x), float64(y), float64(s), 2*(math.Pi/180), 0)
		cr.Fill()
	})
	da.QueueDraw()
}

func finalDraw(win *gtk.Window, main_menu *gtk.Box, da *gtk.DrawingArea, lose_box *gtk.Box, x_int, y_int, s_int int ){
	glib.IdleAdd(func (){
		clearWin(win)
		win.Add(lose_box)
	})



	time.Sleep(2*time.Second)
	
	glib.IdleAdd(func (){
		da.Connect("draw", func(da *gtk.DrawingArea, cr *cairo.Context) {
			cr.SetSourceRGB(255, 255, 255)
			cr.Rectangle(0, 0, 1000, 1000)
			cr.Fill()
		})
		da.QueueDraw()

		clearWin(win)
		win.Add(main_menu)
	})
}


func main(){
	log.Println("Starting...")
	if len(os.Args)<2{
		log.Println("Adress needed")
		os.Exit(1)
	}
	

	var conn net.Conn


	for {
		c, err := net.Dial("tcp", os.Args[1])
		conn = c
		if err!=nil{
			log.Println(err)
			time.Sleep(time.Second*3)
		}else{
			break
		}
	}
	
	log.Println("Connected succesfully")
	defer conn.Close()

	gtk.Init(nil)


	builder, err := gtk.BuilderNewFromFile("ui.glade")
	if err != nil {
		log.Fatal(err)
	}
	win_obj, _ := builder.GetObject("TopLevel")
	main_menu_obj, _ := builder.GetObject("main_menu_box")
	sign_in_obj, _ := builder.GetObject("sign_in_menu_box")
	nickname_entry_obj, _:=builder.GetObject("nickname_entry")
	eror_label_obj, _ := builder.GetObject("eror_message_label")
	play_but_obj, _ := builder.GetObject("play_but")
	greets_label_obj, _ := builder.GetObject("greets_label")
	game_box_obj, _ := builder.GetObject("game_box")
	enemy_label_obj, _ := builder.GetObject("enemy_label")
	game_drawing_area_obj, _ := builder.GetObject("game_drawing_area")
	lose_box_obj, _ := builder.GetObject("lose_box")
	win_box_obj, _ := builder.GetObject("win_box")
	take_but_obj, _ := builder.GetObject("take_but")


	
	win := win_obj.(*gtk.Window)
	win.Connect("destroy", func() { 
		gtk.MainQuit()
	})

	win.Add(sign_in_obj.(*gtk.Box))


	nickname := ""
	apply_but_clicked := func (){
		nickname, _ = nickname_entry_obj.(*gtk.Entry).GetText()
		header:= []byte{1}
		header = append(header, []byte(nickname)...)
		
		_, err := conn.Write(header)
		if err!=nil{log.Panic(err)}

		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)

		if string(buffer[:n])!="200"{
			eror_label_obj.(*gtk.Label).SetText(string(buffer[:n]))
			return
		}

		greets_label_obj.(*gtk.Label).SetText(fmt.Sprintf("Hello %v!", nickname))
		clearWin(win)
		win.Add(main_menu_obj.(*gtk.Box))
	}

	if len(os.Args)>=3{
		nickname_entry_obj.(*gtk.Entry).SetText(os.Args[2])
		apply_but_clicked()
	}


	play_but:=play_but_obj.(*gtk.Button)
	isPlayBut:=true
	var (
		x_int int
		y_int int
		s_int int
	)
	play_but_clicked := func () {
		if isPlayBut{
			isPlayBut=false

			play_but.SetLabel("Searching...")
			_, err:= conn.Write([]byte{2})
			if err!=nil{log.Panic(err)}

			go func() {
				buffer:=make([]byte, 1024)
				n, err:=conn.Read(buffer)
				if err!=nil{return}

				if buffer[0]!=200{log.Panic("ERROR")}


				glib.IdleAdd(func () {
					take_but_obj.(*gtk.Button).SetSensitive(true)
					play_but.SetLabel("Play")
					isPlayBut=true	
				})

				glib.IdleAdd(func () {
					clearWin(win)
					win.Add(game_box_obj.(*gtk.Box))
				})	
				a:=[]byte{4}
				
				_, err= conn.Write(append(a, nickname...))
				n, err=conn.Read(buffer)
				
				loging, xg, yg, sg:=unPackGameHeader(buffer[:n])

				enemy_login, x, y, s := string(loging), string(xg), string(yg), string(sg)
				
				x_int, _ = strconv.Atoi(x)
				y_int, _ = strconv.Atoi(y)
				s_int, _ = strconv.Atoi(s)


				go func () {
					for {
						buffer:=make([]byte, 1024)
						n, err := conn.Read(buffer)
						if err!=nil{log.Println(err);return}

						if string(buffer[:n]) == "ENEMY_TAKED"{

							game_drawing_area_obj.(*gtk.DrawingArea).Connect("draw", func(da *gtk.DrawingArea, cr *cairo.Context) {
								cr.SetSourceRGB(0, 255, 0)
								cr.Arc(float64(x_int), float64(y_int), float64(s_int), 2*(math.Pi/180), 0)
								cr.Fill()
							})
							
							game_drawing_area_obj.(*gtk.DrawingArea).QueueDraw()
						
						}else if string(buffer[:n])=="WIN"{
							
							finalDraw(win, main_menu_obj.(*gtk.Box), game_drawing_area_obj.(*gtk.DrawingArea), win_box_obj.(*gtk.Box), x_int, y_int, s_int)

							break
						}else if string(buffer[:n])=="LOSE"{

							finalDraw(win, main_menu_obj.(*gtk.Box), game_drawing_area_obj.(*gtk.DrawingArea), lose_box_obj.(*gtk.Box), x_int, y_int, s_int)
							break
						}else if buffer[0]==200{ // leave from game
							return
						}else{
							log.Panic("ERROR")
						}
					}
				}()
				
				glib.IdleAdd(func () {
					enemy_label_obj.(*gtk.Label).SetText(fmt.Sprintf("Your enemy: %v", enemy_login))
				})

				drawTarget(game_drawing_area_obj.(*gtk.DrawingArea), x, y, s)

			}()
		}else{
			play_but.SetLabel("Play")
			readStop(conn)
			_, err:= conn.Write([]byte{3})
			if err!=nil{log.Panic(err)}
			isPlayBut=true
		}
	}
	first := true
	take_but_clicked:=func () {
		take_but_obj.(*gtk.Button).SetSensitive(false)
		_, err:=conn.Write([]byte{5})
		if err!=nil{log.Panic(err)}

		SignalHandler.Lock()
		if !first{game_drawing_area_obj.(*gtk.DrawingArea).HandlerDisconnect(SignalHandler.val);first=false}
		SignalHandler.val = game_drawing_area_obj.(*gtk.DrawingArea).Connect("button-press-event", func(da *gtk.DrawingArea, event *gdk.Event) {
			fmt.Println("SDADSAD")
			EventButton := gdk.EventButtonNewFromEvent(event)
			xg, yg := EventButton.X(), EventButton.Y()
			if (xg-float64(x_int))*(xg-float64(x_int))+(yg-float64(y_int))*(yg-float64(y_int))<=float64(s_int*s_int){
				p, _ := json.Marshal([]float64{xg, yg})
				conn.Write(append([]byte{6}, p...))
			}
			first = false

		})
		SignalHandler.Unlock()
	}

	leave_but_clicked:= func () {
		_, err := conn.Write([]byte{7})
		if err!=nil{log.Panic(err)}
		
		readStop(conn)

		buffer:=make([]byte, 1024)
		_, err=conn.Read(buffer)
		if err!=nil{log.Println(err)}
		
		if buffer[0] != 200{log.Panic("ERROR")}

		game_drawing_area_obj.(*gtk.DrawingArea).Connect("draw", func(da *gtk.DrawingArea, cr *cairo.Context) {
			cr.SetSourceRGB(255, 255, 255)
			cr.Rectangle(0, 0, 1000, 1000)
			cr.Fill()
		})
		game_drawing_area_obj.(*gtk.DrawingArea).QueueDraw()
	

		
		clearWin(win)
		win.Add(main_menu_obj.(*gtk.Box))
	}
	// lobbies_but_clicked := func () {
	// 	_, err:= c.Write()
	// }

	exit := func () {
		gtk.MainQuit()
	}
	

	
	
	handlers := make(map[string]any)
	handlers["apply_but_clicked"]=apply_but_clicked
	handlers["play_but_clicked"]=play_but_clicked
	handlers["take_but_clicked"]=take_but_clicked
	handlers["leave_but_clicked"]=leave_but_clicked
	handlers["exit_but_clicked"]=exit
	builder.ConnectSignals(handlers)
	
	win.ShowAll()

	gtk.Main()
}

