package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"path"
	"src/gwp/Chapter_2_Go_ChitChat/chitchat/data"
	"strconv"
	"strings"
	"time"
)

type Item struct {
	Id     int
	Name   string
	Price  float32
	Left   int
	Remark string
	Image  string
}

type ShoppingCar struct {
	Id          int
	ItemId      string
	ItemName    string
	Nums        int
	Prices      float32
	SinglePrice float32
}

func (s ShoppingCar) String() string {
	return fmt.Sprintf("商品: %v 数量: %v 单价:%v 总价:%v \n", s.ItemName, s.Nums, s.SinglePrice, s.Prices)
}

type Order struct {
	Id          int
	Uuid        string
	Phone       string
	ItemName    string
	Prices      float32
	CreateDate  time.Time
	Itemid      int
	Address     string
	Sessionuuid string
}

var Db *sql.DB

func init() {
	var err error
	Db, err = sql.Open("mysql", "root:Lalala123#@tcp(localhost:3306)/littlemouseshopping?charset=utf8&parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	return
}

func noescape(s string) template.HTML {
	return template.HTML(s)
}
func main() {
	// FileServer返回一个使用FileSystem接口root提供文件访问服务的HTTP处理器
	//file, _ := exec.LookPath(os.Args[0])
	//得到全路径，比如在windows下E:\\golang\\test\\a.exe
	//path, _ := filepath.Abs(file)
	//rst := filepath.Dir(path)
	//http.Handle("/", http.FileServer(http.Dir(rst)))
	mux := http.NewServeMux()

	files := http.FileServer(http.Dir("public"))
	mux.Handle("/static/", http.StripPrefix("/static/", files))

	mux.HandleFunc("/index", index)
	mux.HandleFunc("/EditList", editList)
	mux.HandleFunc("/shoppingCar", shoppingCar)
	mux.HandleFunc("/orderRecord", orderRecord)
	mux.HandleFunc("/order", order)

	mux.HandleFunc("/ItemsImport", itemsImport)

	//这里是保存的信息的方法
	mux.HandleFunc("/SaveItem", saveItem)
	mux.HandleFunc("/addToCar", addToCar)
	mux.HandleFunc("/GetItem", getItem)

	mux.HandleFunc("/UpdateItem", updateItem)
	mux.HandleFunc("/LeftToCar", leftToCar)

	//http.ListenAndServe(":8000", nil)
	server := http.Server{
		Addr:           "0.0.0.0:8000",
		Handler:        mux,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 32,
	}
	server.ListenAndServe()
}

func leftToCar(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	value := request.FormValue("b")

	cookie, err := request.Cookie("_cookie")
	if err != nil {
		uuid := CreateUUID()
		//在session表中插入会话信息 作为凭证
		cookie = &http.Cookie{
			Name:     "_cookie",
			Value:    uuid,
			HttpOnly: true,
		}
		//往浏览器客户端中插入cookie信息 作为凭证
		http.SetCookie(writer, cookie)
		err = nil
	}

	Id := request.FormValue("id")
	parseInt, _ := strconv.ParseInt(Id, 10, 32)
	row := Db.QueryRow("select count(*) from `shoppingcar` where itemid = ? and uuid =? and flag =1 and nums>0", Id, cookie.Value)
	count := 0
	row.Scan(&count)
	if count == 0 {
		if value == "1" {
			//http.Redirect(writer, request, "/index", http.StatusTemporaryRedirect)
			//204 no content 一般在只需要从客户端往服务器发送信息,而对客户端不需要发送新信息内容的情况下使用
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(writer, request, "/shoppingCar", http.StatusTemporaryRedirect)
		return
	}
	_, err = Db.Exec("update `shoppingcar` set nums = nums-1 where itemid = ? and uuid =? and flag=1", parseInt, cookie.Value)
	if err != nil {
		writer.Write([]byte("nums -1 failed ..."))
		writer.Write([]byte(err.Error()))
		return
	}
	if value == "1" {
		//http.Redirect(writer, request, "/index", http.StatusTemporaryRedirect)
		//204 no content 一般在只需要从客户端往服务器发送信息,而对客户端不需要发送新信息内容的情况下使用
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(writer, request, "/shoppingCar", http.StatusTemporaryRedirect)
}

func updateItem(writer http.ResponseWriter, request *http.Request) {
	Code := request.FormValue("code")
	if Code != "ljs" {
		writer.Write([]byte("验证码验证失败 别调皮喔~么么哒"))
		return
	}
	request.ParseForm()
	Name := request.FormValue("Name")
	Price := request.FormValue("Price")
	Id := request.FormValue("Id")
	floatPrice, err2 := strconv.ParseFloat(Price, 32)
	if err2 != nil {
		writer.Write([]byte(err2.Error()))
		writer.Write([]byte("商品价格输入格式有误,请重试! "))
		return
	}

	Left := request.FormValue("Left")
	IntLeft, err2 := strconv.ParseInt(Left, 10, 32)
	if err2 != nil {
		writer.Write([]byte(err2.Error()))
		writer.Write([]byte("商品库存输入格式有误,请重试! "))
		return
	}

	Remark := request.FormValue("Remark")
	_, err := Db.Exec("update item set `Name`=? , `Price`=?, `left`=?,`Remark`=? where id=?", Name, floatPrice, IntLeft, Remark, Id)
	if err != nil {
		writer.Write([]byte(err.Error()))
		return
	}
	http.Redirect(writer, request, "/index", http.StatusTemporaryRedirect)
}

func getItem(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	Id := request.FormValue("id")
	item := new(Item)
	err := Db.QueryRow("select `Id`,`Name`,`Price`,`Left`,`Remark` from `item` where `Id`=?", Id).Scan(&item.Id, &item.Name, &item.Price, &item.Left, &item.Remark)
	if err != nil {
		writer.Write([]byte(err.Error()))
		return
	}
	t := template.Must(template.New("ItemsEdit.html").ParseFiles("ItemsEdit.html"))
	t.Execute(writer, item)
}

func addToCar(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	value := request.FormValue("b")

	cookie, err := request.Cookie("_cookie")
	if err != nil {
		uuid := CreateUUID()
		//在session表中插入会话信息 作为凭证
		cookie = &http.Cookie{
			Name:     "_cookie",
			Value:    uuid,
			HttpOnly: true,
		}
		//往浏览器客户端中插入cookie信息 作为凭证
		http.SetCookie(writer, cookie)
		err = nil
	}

	Id := request.FormValue("id")
	parseInt, _ := strconv.ParseInt(Id, 10, 32)
	query, err := Db.Query("select id from `shoppingcar` where itemid = ? and uuid =? and flag =1 ", Id, cookie.Value)
	count := 0
	for query.Next() {
		count++
	}
	if count == 0 {
		_, err := Db.Exec("insert into `shoppingcar` (`itemid`,`nums`,`uuid`,`flag`) values(?,?,?,1)", parseInt, 1, cookie.Value)
		if err != nil {
			writer.Write([]byte("add to shoppingCar failed"))
			writer.Write([]byte(err.Error()))
		}
		if value == "1" {
			//http.Redirect(writer, request, "/index", http.StatusTemporaryRedirect)
			//204 no content 一般在只需要从客户端往服务器发送信息,而对客户端不需要发送新信息内容的情况下使用
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(writer, request, "/shoppingCar", http.StatusTemporaryRedirect)
		return
	}
	_, err = Db.Exec("update `shoppingcar` set nums = nums+1 where itemid = ? and uuid =? and flag=1", parseInt, cookie.Value)
	if err != nil {
		writer.Write([]byte("nums +1 failed ..."))
		writer.Write([]byte(err.Error()))
		return
	}

	if value == "1" {
		//http.Redirect(writer, request, "/index", http.StatusTemporaryRedirect)
		//204 no content 一般在只需要从客户端往服务器发送信息,而对客户端不需要发送新信息内容的情况下使用
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(writer, request, "/shoppingCar", http.StatusTemporaryRedirect)
}

func saveItem(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	request.ParseMultipartForm(1 << 32)
	_, header, err3 := request.FormFile("file")
	if err3 != nil {
		writer.Write([]byte("上传图片失败 请重试"))
		writer.Write([]byte(err3.Error()))
		return
	}
	open, err3 := header.Open()
	if err3 != nil {
		writer.Write([]byte("打开图片失败 请重试"))
		writer.Write([]byte(err3.Error()))
		return
	}
	filename := header.Filename
	defer open.Close()

	str, _ := os.Getwd()

	fileName := path.Join(str, "public", filename)
	newfile, err3 := os.Create(fileName)
	if err3 != nil {
		writer.Write([]byte("创建服务器图片文件失败 请重试"))
		writer.Write([]byte(err3.Error()))
		return
	}
	_, err3 = io.Copy(newfile, open)
	if err3 != nil {
		writer.Write([]byte("拷贝图片失败 请重试"))
		writer.Write([]byte(err3.Error()))
		return
	}
	defer newfile.Close()

	Name := request.FormValue("Name")
	Price := request.FormValue("Price")
	floatPrice, err2 := strconv.ParseFloat(Price, 32)
	if err2 != nil {
		writer.Write([]byte(err2.Error()))
		writer.Write([]byte("商品价格输入格式有误,请重试! "))
		return
	}

	Left := request.FormValue("Left")
	IntLeft, err2 := strconv.ParseInt(Left, 10, 32)
	if err2 != nil {
		writer.Write([]byte(err2.Error()))
		writer.Write([]byte("商品库存输入格式有误,请重试! "))
		return
	}
	Remark := request.FormValue("Remark")
	//Image := request.FormValue("Image")
	_, err := Db.Exec("insert into item(`name`, `price`, `left`,`remark`,`image`) VALUES (?,?,?,?,?)", Name, floatPrice, IntLeft, Remark, filename)
	if err != nil {
		writer.Write([]byte(err.Error()))
		return
	}
	http.Redirect(writer, request, "/index", http.StatusTemporaryRedirect)
}

func itemsImport(writer http.ResponseWriter, request *http.Request) {
	t := template.Must(template.New("ItemsImport.html").ParseFiles("ItemsImport.html"))
	t.Execute(writer, nil)
}

func order(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	phone := request.FormValue("PhoneNum")
	Address := request.FormValue("Address")

	cookie, err := request.Cookie("_cookie")
	if err != nil {
		uuid := CreateUUID()
		//在session表中插入会话信息 作为凭证
		cookie = &http.Cookie{
			Name:     "_cookie",
			Value:    uuid,
			HttpOnly: true,
		}
		//往浏览器客户端中插入cookie信息 作为凭证
		http.SetCookie(writer, cookie)
		err = nil
	}
	query, err := Db.Query("select s.`Id`,i.`Name`,s.`Nums`,i.`Price`,s.`ItemId` from `shoppingcar` s join `item` i on s.ItemId=i.Id  where s.uuid = ? and s.flag =1 and s.`Nums`>0", cookie.Value)
	if err != nil {
		writer.Write([]byte("查看购物车失败 "))
		writer.Write([]byte(err.Error()))
		return
	}

	uuid := CreateUUID()
	buffer := bytes.Buffer{}

	var totalSum float32
	for query.Next() {
		s := new(ShoppingCar)
		query.Scan(&s.Id, &s.ItemName, &s.Nums, &s.SinglePrice, &s.ItemId)
		s.Prices = s.SinglePrice * float32(s.Nums)
		totalSum += s.Prices
		//添加到order中
		_, err := Db.Exec("INSERT INTO `order`(uuid,phone,itemid,prices,createtime,address,sessionuuid) VALUES(?,?,?,?,?,?,?)", uuid, phone, s.ItemId, s.Prices, time.Now().Add(8*time.Hour), Address, cookie.Value)
		if err != nil {
			writer.Write([]byte("下单失败 请重试"))
			writer.Write([]byte(err.Error()))
			return
		}
		buffer.WriteString(s.String())
	}

	buffer.WriteString(fmt.Sprintf("-------------------------------------------------\n"))
	buffer.WriteString(fmt.Sprintf("小票合计: %v\n", totalSum))

	_, err = Db.Exec("delete from `shoppingcar` where uuid = ? and flag=1", cookie.Value)
	if err != nil {
		writer.Write([]byte("清空购物车失败 请重试"))
		writer.Write([]byte(err.Error()))
		return
	}

	// 加载配置文件，登录至邮箱
	config := LoadConfig("./config.json")

	//flag.Usage = flagUsage
	//
	//to := flag.String("To", config.Email, "1534615595@qq.com")
	//title := flag.String("title", fmt.Sprintf("%v-%v", phone, Address), "你好")
	//
	//content := flag.String("content", buffer.String(), "今天心情很好")

	//flag.Parse()

	title := fmt.Sprintf("%v-%v", phone, Address)
	msg := &Msg{
		//Tmail:   config.Email,
		Tmail:   "1534615595@qq.com",
		Title:   title,
		Content: buffer.String(),
	}

	if config.Email != "" && title != "" && buffer.String() != "" {
		SendMail(config, msg)
	} else {
		panic("to,title,content can't be null!")
	}

	http.Redirect(writer, request, "/orderRecord", http.StatusTemporaryRedirect)
}

func orderRecord(writer http.ResponseWriter, request *http.Request) {
	cookie, err := request.Cookie("_cookie")
	t := template.Must(template.New("orderRecord.html").ParseFiles("orderRecord.html"))
	if err != nil {
		uuid := CreateUUID()
		//在session表中插入会话信息 作为凭证
		cookie = &http.Cookie{
			Name:     "_cookie",
			Value:    uuid,
			HttpOnly: true,
		}
		//往浏览器客户端中插入cookie信息 作为凭证
		http.SetCookie(writer, cookie)
		err = nil
	}
	m := make([]Order, 0)
	query, err := Db.Query("SELECT `order`.id,`order`.uuid,`order`.phone,`item`.`Name`,`order`.prices,`order`.createtime FROM `order` join `item` on `order`.itemId = `item`.Id where `sessionuuid` = ? ORDER BY createtime desc  ", cookie.Value)
	if err != nil {
		writer.Write([]byte("查看历史记录失败 "))
		writer.Write([]byte(err.Error()))
		return
	}

	for query.Next() {
		s := new(Order)
		query.Scan(&s.Id, &s.Uuid, &s.Phone, &s.ItemName, &s.Prices, &s.CreateDate)
		m = append(m, *s)
	}

	t.Execute(writer, m)
}

func shoppingCar(writer http.ResponseWriter, request *http.Request) {
	cookie, err := request.Cookie("_cookie")
	if err != nil {
		uuid := CreateUUID()
		//在session表中插入会话信息 作为凭证
		cookie = &http.Cookie{
			Name:     "_cookie",
			Value:    uuid,
			HttpOnly: true,
		}
		//往浏览器客户端中插入cookie信息 作为凭证
		http.SetCookie(writer, cookie)
		err = nil
	}
	m := make([]ShoppingCar, 0)
	query, err := Db.Query("select s.`Id`,i.`Name`,s.`Nums`,i.`Price`,s.`ItemId` from `shoppingcar` s join `item` i on s.ItemId=i.Id  where s.uuid = ? and s.flag =1 and s.`Nums`>0", cookie.Value)
	if err != nil {
		writer.Write([]byte("查看购物车失败 "))
		writer.Write([]byte(err.Error()))
		return
	}
	for query.Next() {
		s := new(ShoppingCar)
		query.Scan(&s.Id, &s.ItemName, &s.Nums, &s.SinglePrice, &s.ItemId)
		s.Prices = s.SinglePrice * float32(s.Nums)
		m = append(m, *s)
	}
	t := template.Must(template.New("shoppingCar.html").ParseFiles("shoppingCar.html"))
	t.Execute(writer, m)
}

func editList(writer http.ResponseWriter, request *http.Request) {
	m := make([]Item, 0)

	query, err := Db.Query("select * from `item` ")
	if err != nil {
		writer.Write([]byte("查找商品出错"))
		writer.Write([]byte(err.Error()))
		return
	}
	for query.Next() {
		i := new(Item)
		query.Scan(&i.Id, &i.Name, &i.Price, &i.Left, &i.Remark, &i.Image)
		m = append(m, *i)
	}
	t := template.Must(template.New("EditList.html").ParseFiles("EditList.html"))
	t.Execute(writer, m)
}

func index(writer http.ResponseWriter, request *http.Request) {
	m := make([]Item, 0)

	query, err := Db.Query("select * from `item` where `left` >0")
	if err != nil {
		writer.Write([]byte("查找商品出错"))
		writer.Write([]byte(err.Error()))
		return
	}
	for query.Next() {
		i := new(Item)
		query.Scan(&i.Id, &i.Name, &i.Price, &i.Left, &i.Remark, &i.Image)
		m = append(m, *i)
	}
	t := template.Must(template.New("shoppingList.html").Funcs(template.FuncMap{"noescape": noescape}).ParseFiles("shoppingList.html"))
	t.Execute(writer, m)
}

func CreateUUID() (uuid string) {
	u := new([16]byte)
	rand.Seed(time.Now().UnixNano())
	_, err := rand.Read(u[:])
	if err != nil {
		log.Fatalln("Cannot generate UUID", err)
	}

	// 0x40 is reserved variant from RFC 4122
	u[8] = (u[8] | 0x40) & 0x7F
	// Set the four most significant bits (bits 12 through 15) of the
	// time_hi_and_version field to the 4-bit version number.
	u[6] = (u[6] & 0xF) | (0x4 << 4)
	uuid = fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
	return
}

type Config struct {
	Email      string `json:"email"`      //账号
	Name       string `json:"name"`       //发送者名字
	Password   string `json:"password"`   //邮箱授权码
	Mailserver string `json:"mailserver"` //邮件服务器
	Port       string `json:"port"`       //服务器端口
}

// Msg 发送邮件信息
type Msg struct {
	Tmail   string
	Title   string
	Content string
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (config *Config) {
	// 读取配置文件
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}
	// 初始化用户信息
	config = &Config{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	return config
}

func SendMail(config *Config, ms *Msg) {
	auth := smtp.PlainAuth("", config.Email, config.Password, config.Mailserver)

	to := []string{ms.Tmail, "2899575553@qq.com", "1541577645@qq.com"} //接收用户
	user := config.Email
	nickname := config.Name

	subject := ms.Title
	content_type := "Content-Type: text/plain; charset=UTF-8"
	body := ms.Content
	msg := "To:" + strings.Join(to, ",") + "\r\nFrom: "
	msg += nickname + "<" + user + ">\r\nSubject: " + subject
	msg += "\r\n" + content_type + "\r\n\r\n" + body

	server := func(serverName, port string) string {

		var buffer bytes.Buffer

		buffer.WriteString(serverName)
		buffer.WriteString(":")
		buffer.WriteString(port)

		return buffer.String()

	}(config.Mailserver, config.Port)

	// 发送邮件
	err := smtp.SendMail(server, auth, user, to, []byte(msg))
	if err != nil {
		fmt.Printf("send mail error:%v\n", err)
	}

	// fmt.Println(server)
	fmt.Println(msg, "\n")
	// fmt.Printf("%v\n", auth)
}

type Session struct {
	Id        int
	Uuid      string
	Email     string
	UserId    int
	CreatedAt time.Time
}

func session(writer http.ResponseWriter, request *http.Request) (sess data.Session, err error) {
	cookie, err := request.Cookie("_cookie")
	if err == nil {
		sess = data.Session{Uuid: cookie.Value}
		if ok, _ := sess.Check(); !ok {
			err = errors.New("Invalid session")
		}
	}
	return
}

func flagUsage() {

	usageText := `Usage mailTo [OPTION]
					Usage parameter:

  -To      		 default: yourself
  -title         default: yourName
  -content       default: Hello`

	fmt.Fprintf(os.Stderr, "%s\n", usageText)
}
