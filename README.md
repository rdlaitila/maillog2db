pfmaillog2db
============

Capture Postfix Mail logs to Database

# Purpose

pfmaillog2db is a golang program designed to monitor your postfix maillog and push log entries to a mysql database. pfmaillog2db captures the following data from postfix logs:

* Raw log entries are inserted into a single table
* Messages and delivery queues are split and tracked among multiple tables
* Connecting Clients are tracked. 

Please create a github issue or submit a pull request if you would like to see the data parsed/handled in any other way, and we will consider making any adjustments.

# Dependencies

You can download any of the pre-compiled binaries from the **builds** folder. https://github.com/admin36/pfmaillog2db/tree/master/builds

If you are going to compile from source, the following dependencies exist:

* github.com/ActiveState/tail
* github.com/go-sql-driver/mysql
* github.com/jinzhu/gorm

Please review the source imports from https://github.com/admin36/pfmaillog2db/blob/master/src/github.com/admin36/pfmaillog2db/pfmaillog2db.go for more details

# Usage

Once you have the binary available either pre-compiled or source-compiled, usage is as follows:




