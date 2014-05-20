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

Pfmaillog2db also requires a mysql database. No other databases are supported at this time. 

# Example Usage

Once you have the binary available either pre-compiled or source-compiled, usage is as follows (assuming installed under /opt/pfmaillog2db):

```bash
/opt/pfmaillog2db/pfmaillog2db -dbhost 10.10.0.1 -dbport 3306 -dbname pfmaillog2db -dbuser pfmaillog2db -dbpass MySecretPassword -maillog /var/log/maillog
```

Once the binary is started, it will parse any existing entries and watch for any new ones. 

for a full list of CLI options, use the following:

```
/opt/pfmaillog2db/pfmaillog2db -h
```

# Database Schema

The database schema is automatically created by the program upon startup. ensure your database user has **CREATE TABLE** permissions on the database. 

# Logging

The program log file by default is placed next to the binary location (ex: /opt/pfmaillog2db/pfmaillog2db > /opt/pfmaillog2db/pfmaillog2db.log). If you would like to change the logging location, simply supply the -logfile CLI switch:

```
/opt/pfmaillog2db/pfmaillog2db -logfile /var/log/pfmaillog2db.log
```

# Performance and safety

No guarantees are being made on performance of this program. It is currently being tested within a production environment to gauge real-world usage and statistics will be released once they become available.

The program was coded with basic safety in mind. No interference with a running postfix installation have been observed. Please feel encouraged to open a Github issue regarding any performance observations or program crashes.


