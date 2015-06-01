/*
*   Copyright 2014 Regan Daniel Laitila
*
*   Licensed under the Apache License, Version 2.0 (the "License");
*   you may not use this file except in compliance with the License.
*   You may obtain a copy of the License at
*
*       http://www.apache.org/licenses/LICENSE-2.0
*
*   Unless required by applicable law or agreed to in writing, software
*   distributed under the License is distributed on an "AS IS" BASIS,
*   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*   See the License for the specific language governing permissions and
*   limitations under the License.
*/

package main

import(
    "fmt"
    "time"
    "log"
    "os"
    "flag"
    "regexp"
    "strings"
    //"database/sql"
    "github.com/ActiveState/tail"
    _ "github.com/go-sql-driver/mysql"
    "github.com/jinzhu/gorm"
)

/*
* Holds configuration flags passed from cli
*/
type Config struct {
    maillog string
    logfile string
    dbhost string
    dbport int
    dbuser string
    dbpass string
    dbname string
    debug bool
}

/*
* Holds our regex constants
*/
const (
    //this regex matches the timestamp, host, process and pid from the line entry
    entry_firstpart_regex_str string = `([a-zA-Z]{1,3}\s[0-9]{2}\s[0-9]{2}:[0-9]{2}:[0-9]{2})\s([\S]+)\s([\S]+)\[([0-9]{1,})\]:`

    //this regex matches the smtpd client log entry
    smtpd_regex1_str string = `([a-zA-Z0-9]{11}):\sclient=(.*)`

    //this regex matches the smtp log entry
    smtp_regex1_str string = `([a-zA-Z0-9]{11}|NOQUEUE):\sto=(.*?),\srelay=(.*?),\sdelay=(.*?),\sdelays=(.*?),\sdsn=(.*?),\sstatus=(.*?)\s(.*)`

    //this regex matches the qmgr log entry
    qmgr_regex1_str string = `([a-zA-Z0-9]{11}):\sfrom=(.*?),\ssize=([0-9]{1,}),\snrcpt=([0-9]{1,})\s(.*)`

    //this regex matches the cleanup message-id log entry
    cleanup_regex1_str string = `([a-zA-Z0-9]{11}):\smessage-id=(.*)`
)

/*
* ORM Struct Types
*/
type Pfmaillog2dbLog struct {
    Id int64
    LogTimestamp time.Time
    LogMailhost string `sql:"type:varchar(100);"`
    LogProcess string `sql:"type:varchar(100);"`
    LogProcessid string `sql:"type:varchar(100);"`
    LogMessage string `sql:"type:varchar(500);"`
    RowCreatedAt time.Time
    RowUpdatedAt time.Time
}
type Pfmaillog2dbClient struct {
    Id int64
    Client string `sql:"type:varchar(500);"`
    ClientRdns string `sql:"type:varchar(255);"`
    ClientAddr string `sql:"type:varchar(50);"`
    ClientLastseen time.Time
    RowCreatedAt time.Time
    RowUpdatedAt time.Time
}
type Pfmaillog2dbMessage struct {
    Id int64
    MessageTimestamp time.Time
    MessageMailhost string `sql:"type:varchar(255);"`
    MessageQueueid string `sql:"type:varchar(11);"`
    MessageFrom string `sql:"type:varchar(100);"`
    MessageSize string `sql:"type:varchar(50);"`
    MessageNrcpt string `sql:"type:varchar(50);"`
    MessageClient string `sql:"type:varchar(500);"`
    MessageStatusext string
    MessageId string `sql:"type:varchar(500);"`
    RowCreatedAt time.Time
    RowUpdatedAt time.Time
}
type Pfmaillog2dbDelivery struct {
    Id int64
    DeliveryTimestamp time.Time
    DeliveryQueueid string `sql:"type:varchar(11);"`
    DeliveryTo string `sql:"type:varchar(100);"`
    DeliveryRelay string `sql:"type:varchar(100);"`
    DeliveryDelay string `sql:"type:varchar(50);"`
    DeliveryDelays string `sql:"type:varchar(50);"`
    DeliveryDsn string `sql:"type:varchar(25);"`
    DeliveryStatus string `sql:"type:varchar(50);"`
    DeliveryStatusext string
    RowCreatedAt time.Time
    RowUpdatedAt time.Time
}

/*
* DB and Error global vars
*/
var DBCONN gorm.DB
var ERROR error

/*
* Main Method
*/
func main() {
    cwd, _ := os.Getwd()

    //Setup Command Line Flags
    flag_maillog := flag.String("maillog", "/var/log/maillog", "Path To Maillog. Default: /var/log/maillog")
    flag_logfile := flag.String("logfile", fmt.Sprintf("%v/pfmaillog2db.log", cwd), "Path To Program Logfile")
    flag_dbhost := flag.String("dbhost", "localhost", "Database Host")
    flag_dbport := flag.Int("dbport", 3306, "Database Port")
    flag_dbuser := flag.String("dbuser", "username", "Database Username")
    flag_dbpass := flag.String("dbpass", "password", "Database Password")
    flag_dbname := flag.String("dbname", "databasename", "Database Name")
    flag_debug := flag.Bool("debug", false, "Debug Output. Default: false")
    flag.Parse()

    //Setup Config
    config := Config{*flag_maillog, *flag_logfile, *flag_dbhost, *flag_dbport, *flag_dbuser, *flag_dbpass, *flag_dbname, *flag_debug}

    //Setup Program Log
    logfile, err := os.OpenFile(config.logfile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
    if err !=nil {
        log.Fatal(err)
    } else {
        log.SetOutput(logfile)
        if config.debug {
            log.Println(fmt.Sprintf("Logging To %v", config.logfile))
        }
    }
    defer logfile.Close()

    //Setup DB
    dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?parseTime=True", config.dbuser, config.dbpass, config.dbhost, config.dbport, config.dbname)
    DBCONN, ERROR = gorm.Open("mysql", dsn)
    if ERROR != nil {
        log.Fatal(ERROR)
    }
    DBCONN.DB().SetMaxIdleConns(50)
    DBCONN.DB().SetMaxOpenConns(200)

    //Ensure DB is reachable
    log.Println("Attempting Database Connection")
    ERROR = DBCONN.DB().Ping()
    if ERROR != nil {
        log.Fatal(ERROR)
    } else {
        if config.debug {
            log.Println("Database Connection Successful")
        }
    }

    //ensure db tables exist:
    DBCONN.AutoMigrate(&Pfmaillog2dbLog{})
    DBCONN.AutoMigrate(&Pfmaillog2dbClient{})
    DBCONN.AutoMigrate(&Pfmaillog2dbMessage{})
    DBCONN.AutoMigrate(&Pfmaillog2dbDelivery{})

    //Setup Maillog Tail
    tail_handle, err := tail.TailFile(config.maillog, tail.Config{Follow: true,ReOpen: true})
    if err != nil {
        log.Fatal(err)
    }

    //compile regex
    entry_firstpart_regex := regexp.MustCompile(entry_firstpart_regex_str)
    smtpd_regex1 := regexp.MustCompile(smtpd_regex1_str)
    smtp_regex1 := regexp.MustCompile(smtp_regex1_str)
    qmgr_regex1 := regexp.MustCompile(qmgr_regex1_str)
    cleanup_regex1 := regexp.MustCompile(cleanup_regex1_str)

    //begin watching maillog and parsing entries
    for line := range tail_handle.Lines {
        if entry_firstpart_regex.MatchString(line.Text) == false {
            continue
        }

        entry_firstpart := entry_firstpart_regex.FindAllStringSubmatch(line.Text, -1)

        //strip common entry text from line
        remaining := strings.Trim(strings.Replace(line.Text, entry_firstpart[0][0], "", -1), " ")

        if config.debug {
            fmt.Println("timestamp:", entry_firstpart[0][1])
            fmt.Println("mailhost:", entry_firstpart[0][2])
            fmt.Println("process:", entry_firstpart[0][3])
            fmt.Println("processid:", entry_firstpart[0][4])
            fmt.Println("message:", remaining)
        }

        //add raw log entry to db if not exists
        recordRawLogEntry(entry_firstpart[0][1], entry_firstpart[0][2], entry_firstpart[0][3], entry_firstpart[0][4], remaining)

        switch {
            case smtpd_regex1.MatchString(remaining):
                matches := smtpd_regex1.FindAllStringSubmatch(remaining, -1)

                if config.debug {
                    fmt.Println("queueid:", matches[0][1])
                    fmt.Println("client:", matches[0][2])
                }

                //split client string to rdns/ip and record entry if not exists
                clientsplit_regex := regexp.MustCompile(`(.*?)\[(.*?)\]`)
                csplitmatches := clientsplit_regex.FindAllStringSubmatch(matches[0][2], -1)

                //record client entry
                recordClientEntry(csplitmatches[0][0], csplitmatches[0][1], csplitmatches[0][2], entry_firstpart[0][1])

                //record client in message entry
                recordMessageClientEntry(matches[0][1], matches[0][2])
                break
            case smtp_regex1.MatchString(remaining):
                matches := smtp_regex1.FindAllStringSubmatch(remaining, -1)

                if config.debug {
                    fmt.Println("queueid:", matches[0][1])
                    fmt.Println("to:", matches[0][2])
                    fmt.Println("relay:", matches[0][3])
                    fmt.Println("delay:", matches[0][4])
                    fmt.Println("delays:", matches[0][5])
                    fmt.Println("dsn:", matches[0][6])
                    fmt.Println("status:", matches[0][7])
                    fmt.Println("statusext:", matches[0][8])
                }

                recordDeliveryEntry(
                    entry_firstpart[0][1],
                    matches[0][1],
                    matches[0][2],
                    matches[0][3],
                    matches[0][4],
                    matches[0][5],
                    matches[0][6],
                    matches[0][7],
                    matches[0][8])
                break
            case qmgr_regex1.MatchString(remaining):
                matches := qmgr_regex1.FindAllStringSubmatch(remaining, -1)

                if config.debug {
                    fmt.Println("queueid:", matches[0][1])
                    fmt.Println("from:", matches[0][2])
                    fmt.Println("size:", matches[0][3])
                    fmt.Println("nrcpt:", matches[0][4])
                    fmt.Println("statusext:", matches[0][5])
                }

                recordMessageEntry(
                    entry_firstpart[0][1],
                    entry_firstpart[0][2],
                    matches[0][1],
                    matches[0][2],
                    matches[0][3],
                    matches[0][4],
                    matches[0][5])
                break
            case cleanup_regex1.MatchString(remaining):
                matches := cleanup_regex1.FindAllStringSubmatch(remaining, -1)

                if config.debug {
                    fmt.Println("queueid:", matches[0][1])
                    fmt.Println("message-id:", matches[0][2])
                }

                recordMessageMessageIdEntry(matches[0][1], matches[0][2])
                break
            default:
                if config.debug {
                    fmt.Println("entry matches no available regex", remaining)
                }
                break
        }

        if config.debug {
            fmt.Println("--------------------------------------------------")
        }
    }
}

/*
* Adds a raw log entry into pfmaillog2db_logs table only if it does not yet exist
*/
func recordRawLogEntry(TIMESTAMP string, MAILHOST string, PROCESS string, PROCESSID string, MESSAGE string) {
    var logentries []Pfmaillog2dbLog
    DBCONN.Where(`
        log_timestamp=? and
        log_mailhost=? and
        log_process=? and
        log_processid=? and
        log_message=?`,
        pfdate2golang(TIMESTAMP),
        MAILHOST,
        PROCESS,
        PROCESSID,
        MESSAGE).Find(&logentries)

    if len(logentries) == 0 {
        DBCONN.Save(&Pfmaillog2dbLog{
            RowCreatedAt: time.Now(),
            LogTimestamp: pfdate2golang(TIMESTAMP),
            LogMailhost: MAILHOST,
            LogProcess: PROCESS,
            LogProcessid: PROCESSID,
            LogMessage: MESSAGE})
    }
}

func recordClientEntry(CLIENTSTR string, CLIENTRDNS string, CLIENTIP string, CLIENTLASTSEEN string) {
    var cliententries []Pfmaillog2dbClient
    DBCONN.Where(`
        client=? and
        client_rdns=? and
        client_addr=?`,
        CLIENTSTR,
        CLIENTRDNS,
        CLIENTIP).Find(&cliententries)

    if len(cliententries) == 0 {
        DBCONN.Save(&Pfmaillog2dbClient{
            RowCreatedAt: time.Now(),
            Client: CLIENTSTR,
            ClientRdns: CLIENTRDNS,
            ClientAddr: CLIENTIP,
            ClientLastseen: pfdate2golang(CLIENTLASTSEEN)})
    } else {
        cliententries[0].RowUpdatedAt = time.Now()
        cliententries[0].ClientLastseen = pfdate2golang(CLIENTLASTSEEN)
        DBCONN.Save(&cliententries[0])
    }
}

func recordMessageEntry(TIMESTAMP string, MAILHOST string, QUEUEID string, FROM string, SIZE string, NRCPT string, STATUSEXT string) {
    var messageentries []Pfmaillog2dbMessage
    DBCONN.Where(`
        message_queueid=?
    `, QUEUEID).Find(&messageentries)

    if len(messageentries) == 0 {
        DBCONN.Save(&Pfmaillog2dbMessage{
            RowCreatedAt: time.Now(),
            MessageTimestamp: pfdate2golang(TIMESTAMP),
            MessageMailhost: MAILHOST,
            MessageQueueid: QUEUEID,
            MessageFrom: FROM,
            MessageSize: SIZE,
            MessageNrcpt: NRCPT,
            MessageStatusext:STATUSEXT})
    } else {
        messageentries[0].RowUpdatedAt = time.Now()
        messageentries[0].MessageTimestamp = pfdate2golang(TIMESTAMP)
        messageentries[0].MessageMailhost = MAILHOST
        messageentries[0].MessageFrom = FROM
        messageentries[0].MessageSize = SIZE
        messageentries[0].MessageNrcpt = NRCPT
        messageentries[0].MessageStatusext = STATUSEXT
        DBCONN.Save(&messageentries[0])
    }
}

func recordMessageClientEntry(QUEUEID string, CLIENTSTR string) {
    var messageentries []Pfmaillog2dbMessage
    DBCONN.Where(`
        message_queueid=?
    `, QUEUEID).Find(&messageentries)

    if len(messageentries) == 0 {
        DBCONN.Save(&Pfmaillog2dbMessage{
            RowCreatedAt: time.Now(),
            MessageQueueid: QUEUEID,
            MessageClient: CLIENTSTR})
    } else {
        messageentries[0].RowUpdatedAt = time.Now()
        messageentries[0].MessageClient = CLIENTSTR
        DBCONN.Save(&messageentries[0])
    }
}

func recordMessageMessageIdEntry(QUEUEID string, MESSAGEID string) {
    var messageentries []Pfmaillog2dbMessage
    DBCONN.Where(`
        message_queueid=?
    `, QUEUEID).Find(&messageentries)

    if len(messageentries) == 0 {
        DBCONN.Save(&Pfmaillog2dbMessage{
            MessageQueueid: QUEUEID,
            MessageId: MESSAGEID})
    } else {
        messageentries[0].RowUpdatedAt = time.Now()
        messageentries[0].MessageId = MESSAGEID
        DBCONN.Save(&messageentries[0])
    }
}

func recordDeliveryEntry(TIMESTAMP string, QUEUEID string, TO string, RELAY string, DELAY string, DELAYS string, DSN string, STATUS string, STATUSEXT string) {
    var deliveryentries []Pfmaillog2dbDelivery
    DBCONN.Where(`
        delivery_timestamp=? and
        delivery_queueid=? and
        delivery_to=? and
        delivery_relay=? and
        delivery_delay=? and
        delivery_delays=? and
        delivery_dsn=? and
        delivery_status=? and
        delivery_statusext=?`,
        pfdate2golang(TIMESTAMP),
        QUEUEID,
        TO,
        RELAY,
        DELAY,
        DELAYS,
        DSN,
        STATUS,
        STATUSEXT).Find(&deliveryentries)

    if len(deliveryentries) == 0 {
        DBCONN.Save(&Pfmaillog2dbDelivery{
            RowCreatedAt: time.Now(),
            DeliveryTimestamp: pfdate2golang(TIMESTAMP),
            DeliveryQueueid: QUEUEID,
            DeliveryTo: TO,
            DeliveryRelay: RELAY,
            DeliveryDelay: DELAY,
            DeliveryDelays: DELAYS,
            DeliveryDsn: DSN,
            DeliveryStatus: STATUS,
            DeliveryStatusext: STATUSEXT})
    } else {
        deliveryentries[0].RowUpdatedAt = time.Now()
        deliveryentries[0].DeliveryTimestamp = pfdate2golang(TIMESTAMP)
        deliveryentries[0].DeliveryQueueid = QUEUEID
        deliveryentries[0].DeliveryTo = TO
        deliveryentries[0].DeliveryRelay = RELAY
        deliveryentries[0].DeliveryDelay = DELAY
        deliveryentries[0].DeliveryDelays = DELAYS
        deliveryentries[0].DeliveryDsn = DSN
        deliveryentries[0].DeliveryStatus = STATUS
        deliveryentries[0].DeliveryStatusext = STATUSEXT
        DBCONN.Save(&deliveryentries[0])
    }
}

func pfdate2golang(POSTFIXDATE string) time.Time {
    value := fmt.Sprintf("%v %v", time.Now().Year(), POSTFIXDATE)
    rtime, err := time.Parse("2006 Jan 2 15:04:05", value)
    if err != nil {
        log.Fatal("Error Parsing Time Format: ", value)
    } else {
        return rtime
    }

    return time.Now()
}
