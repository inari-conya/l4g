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

	"net/mail"

	"github.com/go-ini"
	"github.com/scorredoira/email"
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
		Uname   string `ini:"username"`
	} `ini:"[config]"`
}

var logfile map[string]*os.File
var logdir string
var oldpath string
var tnow string

var mini MyIni

var maxsize int64
var user string
var password string
var host string
var sendto string
var subject string
var body string
var username string

func init() {
	maxsize = 10485760 //默认日志文件不超过10M

	/*读取配置文件*/
	content, err := ioutil.ReadFile("l4gconf.ini")
	if err != nil {
		fmt.Println("read l4gconf.ini error:", err)
	}

	err = ini.Unmarshal(content, &mini)
	if err != nil {
		fmt.Println("Unmarshal config file error:", err)
	}

	user = mini.Config.Usr
	password = mini.Config.Passwd
	host = mini.Config.Hst
	sendto = mini.Config.Sndto
	subject = mini.Config.Sbj
	body = mini.Config.Msg
	username = mini.Config.Uname

	/*显示配置文件内容*/
	fmt.Println(mini.Config)
	/*设置日志文件占用磁盘空间上限，但不得低于10K*/
	if mini.Config.MaxSize > 10239 {
		maxsize = mini.Config.MaxSize
	}

	logfile = make(map[string]*os.File)
	/*判断log目录是否存在，不存在则创建该目录*/
	if exist("log") == false {
		err := os.Mkdir("log", os.ModePerm)
		if err != nil {
			fmt.Println("目录创建错误:", err)
		}
	}

	/*根据当前日期创建log子目录*/
	t := time.Now().Day()
	today := strconv.Itoa(t)
	path := "log/" + today

	if exist(path) == false {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			fmt.Println("目录创建错误:", err)
		}
	}
	logdir = path + "/"

	go updatetime()
	go logmonitor()
}

/*判断文件是否存在*/
func exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

/*根据日期更新日志文件写入路径与根据时间更新日志记录时显示的时间*/
func updatetime() {
	for true {
		t := time.Now()
		day := t.Day()
		today := strconv.Itoa(day)
		path := "log/" + today

		if exist(path) == false {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				fmt.Println("Create index error:", err)
				Log("log.log", "ERROR", "Create index error:", err)
			} else {
				oldpath = logdir
				logdir = path + "/"
				go yestodaycloser()
			}
		}
		h, m, s := t.Clock()
		tnow = fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		time.Sleep(1 * time.Second)
	}
}

/*如果日志文件超出MAXSIZE大小，则发送邮件并立即关闭该文件*/
func logmonitor() {
	for true {
		/*遍历日志文件*/
		for v, k := range logfile {
			Fstat, err := os.Stat(v)
			if err != nil {
				fmt.Println("Get file statue error:", err)
				Log("log.log", "ERROR", "Get file statue error:", err)
			}
			Fsize := Fstat.Size()
			//fmt.Println("Size:", Fsize)
			/*判断日志文件大小*/
			if Fsize > maxsize {
				k.Seek(0, 0)        /*将io光标插入文件头*/
				k.Truncate(maxsize) /*砍去超出部分*/
				k.Close()           /*关闭文件，不再写入新内容*/
				i := 0
				for i < 3 {
					Log("sendmail.log", "INFO", "send mail")
					subject += v
					/*发送日志异常告警邮件*/
					//err := sendmail(user, password, host, sendto, subject, body, "html")
					err := sendmail(v)
					if err != nil {
						fmt.Println("send mail error:", err)
						Log("sendmail.log", "ERROR", "send mail error:", err)
					} else {
						break
					}
					i += 1
				}
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
		if exist(logpath) == true {
			err := os.RemoveAll(logpath)
			if err != nil {
				fmt.Println("Delete tomorrow log index error:", err)
				Log("log.log", "ERROR", "Delete tomorrow log index error:", err)
			}
		}
		//time.Sleep(1 * time.Second)
		time.Sleep(60 * time.Second)
	}
}

/*关闭所有已打开的文件*/
func LogClose() {
	for _, k := range logfile {
		k.Close()
	}
	return
}

/*发送邮件*/
/*func sendmail(user, password, host, to, subject, body, mailtype string) error {
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
}*/
func sendmail(filename string) error {
	m := email.NewMessage(subject, body)
	m.From = mail.Address{Name: username, Address: user}
	m.To = strings.Split(sendto, ",")

	// add attachments
	if err := m.Attach(filename); err != nil {
		fmt.Println("Send mail error:", err)
		return err
	}

	// send it
	auth := smtp.PlainAuth("", user, password, strings.TrimSuffix(host, ":25"))
	if err := email.Send(host, auth, m); err != nil {
		fmt.Println("Send mail error:", err)
		return err
	}
	return nil
}

/*关闭昨天的文件*/
func yestodaycloser() {
	/*为防止出现冲突，等待10分钟后再执行关闭操作*/
	time.Sleep(600 * time.Second)
	files, _ := ioutil.ReadDir(oldpath)
	for _, file := range files {
		filename := file.Name()
		fi := oldpath + filename
		if logfile[fi] != nil {
			logfile[fi].Close()
			delete(logfile, fi)
		}
	}
}

func Log(file, lvl string, args ...interface{}) {
	filename := logdir + file
	str := tnow + " [" + lvl + "] " + fmt.Sprint(args...) + "\n"
	go wrlog(filename, str)
	return
}

func Logf(file, lvl, format string, args ...interface{}) {
	filename := logdir + file
	str := tnow + " [" + lvl + "] " + fmt.Sprintf(format, args...) + "\n"
	go wrlog(filename, str)
	return
}

func wrlog(filename, str string) {
	/*判断需要写入的文件是否已经打开*/
	if logfile[filename] == nil {
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("Open file error:", err)
		} else {
			logfile[filename] = f
			_, err := f.Seek(0, 2) /*将io光标插入文件末尾*/
			if err != nil {
				fmt.Println("Set seek to end error:", err)
			}
		}
	}

	/*执行写操作*/
	_, err := io.WriteString(logfile[filename], str)
	if err != nil {
		fmt.Println("Write file error:", err)
	}
}
