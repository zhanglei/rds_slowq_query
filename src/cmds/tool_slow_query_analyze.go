package main

import (
	"bytes"
	"flag"
	"fmt"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"os"
	"os/exec"
	"slow_query"
	"strings"
	"time"
)

var (
	verbose   = flag.Bool("verbose", false, "verbose")
	hourParam = flag.Int64("hour", -1, "hour in utc0")
	conf      = flag.String("conf", "", "database conf file")
)

func main() {
	flag.Parse()
	hour := *hourParam

	conf, err := slow_query.NewConfigWithFile(*conf)
	if err != nil {
		log.ErrorErrorf(err, "NewConfigWithFile failed")
		return
	}

	if hour == -1 {
		hour = int64(time.Now().Hour()) - 1
		if hour < 0 {
			hour = 23
		}
	} else if hour < 0 || hour > 23 {
		log.Printf("Invalid hour: %d", hour)
		return
	}

	// 要下载的mysql slow query
	logFile := fmt.Sprintf("slowquery/mysql-slowquery.log.%d", hour)
	// perl脚本
	mysqlDumpSlow := "scripts/mysqldumpslow.pl"

	var emailContentBlocks []string

	// 下载文件(Online和Offline分开处理)
	for idx, dbs := range [][]string{conf.DatabasesOnline, conf.DatabasesOffline} {
		var slowQueryLogs []string
		for _, db := range dbs {
			content, err := slow_query.DownloadHourlyLogFile(db, logFile, conf)
			if err != nil {
				log.ErrorErrorf(err, "DownloadToPath failed")
				return
			}
			slowQueryLogs = append(slowQueryLogs, content)
		}

		filePath, err := slow_query.SaveToDefaultFormat(slowQueryLogs)
		if err != nil {
			log.ErrorError(err, "SaveToDefaultFormat")
			return
		}

		cmd := exec.Command(mysqlDumpSlow, filePath)

		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()

		// 删除数据
		os.Remove(filePath)

		if err != nil {
			log.ErrorError(err, "error while run command")
			return
		}

		summaries := slow_query.ParseSummaries(out.String(), true)

		if idx == 0 {
			content := slow_query.FormatMail(summaries, 100, *verbose)
			content = fmt.Sprintf(`<span style="color:red;">慢日志文件Online：%s</span><br/>`, logFile) + content
			emailContentBlocks = append(emailContentBlocks, content)
		} else {
			content := slow_query.FormatMail(summaries, 10, *verbose)
			content = fmt.Sprintf(`<span style="color:red;">慢日志文件Offline：%s</span><br/>`, logFile) + content
			emailContentBlocks = append(emailContentBlocks, content)
		}

	}

	slow_query.SendEmail("MySQL慢查询统计", strings.Join(emailContentBlocks, "<hr style='height: 5px;background-color: #03A9F4;'/>"),
		conf.EmailSender, conf.EmailReceivers, conf)
}
