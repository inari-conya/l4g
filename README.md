# l4g
GO高级日志工具
## 使用方法：
### 非格式化日志记录：Log("test.log"(日志文件名), "DEBUG"(日志等级), "log test:", err(...interface{}))
### 格式化日志记录：Logf("test.log"(日志文件名), "ERROR"(日志等级), "%s %v",str, err(...interface{}))
### 关闭所有已经打开日志记录文件：LogClose()
## 功能简介：
### 默认保管一个月的日志
### 邮箱配置、日志文件大小上限，参照l4gconf.ini
### 当日志大小达到上限时，会先发送邮件，然后将io指针指向文件开头，之后截去超出上限部分的内容，最后会关闭该日志文件
### 设置io指针是为了防止在截去多余部分之后，文件关闭之前，会有新内容插入，使得文件大小又超出上限，但由于文件已关闭，所以无法再次截去多余内容，会导致日志工具不停的发送邮件
### l4gconf.ini请放在程序根目录中
## 注：.ini文件因保存不当容易出现空读现象，所以本工具默认会在进程启动时在控制台上打印从.ini文件中读取到的信息，请注意检查
