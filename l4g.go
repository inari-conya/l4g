package l4g

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-ini"
)

type MyIni struct {
	Config struct {
		MaxSize int64  `ini:"logsize"`
		Usr     string `ini:"mailuser"`
		Passwd  string `ini:"smtppassword"`
		Hst     string `ini:"host"`
		Sndto   string `ini:"sendto"`
		Sbj     string `ini:"subject"`
		Msg     string `ini:"message"`
	} `ini:"[config]"`
}

var LogFile map[string]*os.File
var LogDir string
var OldPath string
var Tnow string

var mini MyIni

var MAXSIZE int64
var User string
var Password string
var Host string
var Sendto string
var Subject string
var Body string

func init() {
	MAXSIZE = 10485760 //默认日志文件不超过10M

	/*读取配置文件*/
	content, err := ioutil.ReadFile("l4gconf.ini")
	if err != nil {
		panic(err)
	}

	err = ini.Unmarshal(content, &mini)
	if err != nil {
		panic(err)
	}

	User = mini.Config.Usr
	Password = mini.Config.Passwd
	Host = mini.Config.Hst
	Sendto = mini.Config.Sndto
	Subject = mini.Config.Sbj
	Body = `
	<html>
	<body>
	"` + mini.Config.Msg + `"
	</body>
	</html>
	`

	/*设置日志文件占用磁盘空间上限，但不得低于10K*/
	if mini.Config.MaxSize > 10239 {
		MAXSIZE = mini.Config.MaxSize
	}

	LogFile = make(map[string]*os.File)
	/*判断log目录是否存在，不存在则创建该目录*/
	if Exist("log") == false {
		err := os.Mkdir("log", os.ModePerm)
		if err != nil {
			fmt.Println("目录创建错误:", err)
			os.Exit(1)
		}
	}

	/*根据当前日期创建log子目录*/
	t := time.Now().Day()
	today := strconv.Itoa(t)
	path := "log/" + today

	if Exist(path) == false {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			fmt.Println("目录创建错误:", err)
			os.Exit(1)
		}
	}
	LogDir = path + "/"

	go UpdateTime()
	go LogMonitor()
}

/*判断文件是否存在*/
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

/*根据日期更新日志文件写入路径与根据时间更新日志记录时显示的时间*/
func UpdateTime() {
	for true {
		t := time.Now()
		day := t.Day()
		today := strconv.Itoa(day)
		path := "log/" + today

		if Exist(path) == false {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				Log("log.log", "ERROR", "Create index error:", err)
			} else {
				OldPath = LogDir
				LogDir = path + "/"
				go YestodayCloser()
			}
		}
		h, m, s := t.Clock()
		Tnow = fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		time.Sleep(1 * time.Second)
	}
}

func Log(file, lvl string, args ...interface{}) {
	filename := LogDir + file
	if LogFile[filename] == nil {
		//f, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("Open file error:", err)
		} else {
			LogFile[filename] = f
			_, err := f.Seek(0, 2) /*将io光标插入文件末尾*/
			if err != nil {
				fmt.Println("Set seek to end error:", err)
			}
		}
	}

	str := Tnow + " [" + lvl + "] "
	for _, arg := range args {
		str += fmt.Sprint(arg, " ")
	}
	str += "\n"
	_, err := io.WriteString(LogFile[filename], str)
	if err != nil {
		fmt.Println("Write file error:", err)
	}
	return
}

/*如果日志文件超出MAXSIZE大小，则发送邮件并立即退出应用*/
func LogMonitor() {
	for true {
		/*遍历日志文件*/
		for v, k := range LogFile {
			Fstat, err := os.Stat(v)
			if err != nil {
				Log("log.log", "ERROR", "Get file statue error:", err)
			}
			Fsize := Fstat.Size()
			//fmt.Println("Size:", Fsize)
			/*判断日志文件大小*/
			if Fsize > MAXSIZE {
				i := 0
				for i < 3 {
					Log("sendmail.log", "INFO", "send mail")
					Subject += v
					/*发送日志异常告警邮件*/
					err := SendMail(User, Password, Host, Sendto, Subject, Body, "html")
					if err != nil {
						Log("sendmail.log", "ERROR", "send mail error:", err)
					} else {
						break
					}
					i += 1
				}
				k.Seek(0, 0) /*将io光标插入文件头*/
				k.Truncate(MAXSIZE) /*砍去超出部分*/
				k.Close()/*关闭文件，不再写入新内容*/
			}
		}

		/*判断现在是否已经是晚上10点*/
		hnow, _, _ := time.Now().Clock()
		if hnow != 23 {
			continue
		}
		/*删除明天的目录（如果存在，里面内容就是一个月前的）*/
		tmr := time.Now().AddDate(0, 0, 1)
		tmrday := tmr.Day()
		strday := strconv.Itoa(tmrday)
		logpath := "log/" + strday
		if Exist(logpath) == true {
			err := os.RemoveAll(logpath)
			if err != nil {
				Log("log.log", "ERROR", "Delete tomorrow log index error:", err)
			}
		}
		//time.Sleep(1 * time.Second) 测试用
		time.Sleep(60 * time.Second)
	}
}

/*关闭所有已打开的文件*/
func LogClose() {
	for _, k := range LogFile {
		k.Close()
	}
	return
}

/*发送邮件*/
func SendMail(user, password, host, to, subject, body, mailtype string) error {
	hp := strings.Split(host, ":")
	auth := smtp.PlainAuth("", user, password, hp[0])
	var content_type string
	if mailtype == "html" {
		content_type = "Content-Type: text/" + mailtype + "; charset=UTF-8"
	} else {
		content_type = "Content-Type: text/plain" + "; charset=UTF-8"
	}

	msg := []byte("To: " + to + "\r\nFrom: " + user + "<" + user + ">\r\nSubject: " + subject + "\r\n" + content_type + "\r\n\r\n" + body)
	send_to := strings.Split(to, ";")
	err := smtp.SendMail(host, auth, user, send_to, msg)
	return err
}

/*关闭昨天的文件*/
func YestodayCloser() {
	/*为防止出现冲突，等待10分钟后再执行关闭操作*/
	time.Sleep(600 * time.Second)
	files, _ := ioutil.ReadDir(OldPath)
	for _, file := range files {
		filename := file.Name()
		fi := OldPath + filename
		if LogFile[fi] != nil {
			LogFile[fi].Close()
			delete(LogFile, fi)
		}
	}
}
