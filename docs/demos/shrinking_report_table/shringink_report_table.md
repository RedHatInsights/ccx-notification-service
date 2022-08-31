Optimizing and shrinking `reported` table for CCX Notification Service
====================================================================

Reasons
-------
* Access to `reported` table is very slow on production system
* Even simple `select count(*) from report` can take minutes!
* Table grows over time (new features) up to ~150GB as high limit today
* Will be limiting factor for integration with ServiceLog

Table structure
---------------
* Seems simple enough
    - and it is
    - but devil is in the details as usual

```
postgres=# \d reported ;
                             Table "public.reported"
      Column       |            Type             | Collation | Nullable | Default
-------------------+-----------------------------+-----------+----------+--------
 org_id            | integer                     |           | not null |
 account_number    | integer                     |           | not null |
 cluster           | character(36)               |           | not null |
 notification_type | integer                     |           | not null |
 state             | integer                     |           | not null |
 report            | character varying           |           | not null |
 updated_at        | timestamp without time zone |           | not null |
 notified_at       | timestamp without time zone |           | not null |
 error_log         | character varying           |           |          |
 event_type_id     | integer                     |           | not null |
Indexes:
    "reported_pkey" PRIMARY KEY, btree (org_id, cluster, notified_at)
    "notified_at_desc_idx" btree (notified_at DESC)
    "updated_at_desc_idx" btree (updated_at)
Foreign-key constraints:
    "fk_notification_type" FOREIGN KEY (notification_type) REFERENCES notification_types(id)
    "fk_state" FOREIGN KEY (state) REFERENCES states(id)
    "reported_event_type_id_fkey" FOREIGN KEY (event_type_id) REFERENCES event_targets(id)
```

Table size
----------
* CCX Notification Service is started with 15 minutes frequency
    - i.e. 4 times per hour
* On each run
    - approximatelly 65000 reports are written into the table
* Reports are stored for 8 days
    - limit required for 'cooldown' feature
* Simple math
    - number of records=8 days × 24 hours × 4 (runs per hour) × 65000
    - number of records=8×24×4×65000
    - 49920000 ~ 50 millions records!
* ⇒ "each byte in reports counts!"



`reported` table details
========================

* Let's look at the `reported` table used in production for several months
* We'll measure overall table size
    - real size, not just #records
* We'll measure how fast is `select count(*) from reported`
    - it needs to go through one selected index to compute that value
    - good for measure overall DB shape

Overall table size
------------------

```
postgres=# SELECT pg_size_pretty(pg_total_relation_size('reported'));
 pg_size_pretty
----------------
 28 GB
(1 row)
```

Accessing table indexes
-----------------------
```
postgres=# explain(analyze, buffers, format text) select count(*) from reported;

 Finalize Aggregate  (cost=640314.62..640314.63 rows=1 width=8) (actual time=1750.633..1756.451 rows=1 loops=1)
   Buffers: shared hit=192 read=39748
   ->  Gather  (cost=640314.41..640314.62 rows=2 width=8) (actual time=1750.483..1756.441 rows=3 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         Buffers: shared hit=192 read=39748
         ->  Partial Aggregate  (cost=639314.41..639314.42 rows=1 width=8)
                                (actual time=1738.180..1738.180 rows=1 loops=3)
               Buffers: shared hit=192 read=39748
               ->  Parallel Index Only Scan using updated_at_desc_idx on reported
                   (cost=0.44..590342.78 rows=19588652 width=0)
                   (actual time=1.125..1015.623 rows=15677300 loops=3)
                     Heap Fetches: 0
                     Buffers: shared hit=192 read=39748
 Planning Time: 0.135 ms
 Execution Time: 1756.511 ms
                 ^^^^^^^^^^^
```

Conclusion
----------
* Not super fast
* OTOH for approximately 50M records it is still ok
* Can be make faster
     - by introducing simpler index (ID)
     - that will increase table size by 50 000 000 * sizeof(ID) :)



Table as live entity in the system
==================================
* Content of `reported` table is changing quite rapidly
    - well it depends how we look at it
    - each hour, approximately 65000 × 4 = 260000 reports are created/deleted
    - a lot in terms of changed MB/GB
    - OTOH it is just 0.52% of the whole table 

After one batch run of CCX Notification Service
-----------------------------------------------

* Table size grows a lot

```
postgres=# SELECT pg_size_pretty(pg_total_relation_size('reported'));
 pg_size_pretty
----------------
 56 GB
(1 row)
```

* Auto vacuumer is started in the background
* It affects all access to the `reported` table significantly
     - this is the little devil we talked about!
     - (Aristotle was right: "horror vacui")

```
postgres=# select * from pg_stat_progress_vacuum;
  pid  | datid | datname  | relid |       phase       | heap_blks_total | heap_blks_scanned | heap_blks_vacuumed | index_vacuum_count | max_dead_tuples | num_d
ead_tuples
-------+-------+----------+-------+-------------------+-----------------+-------------------+--------------------+--------------------+-----------------+------
-----------
 14583 | 14450 | postgres | 49851 | vacuuming indexes |         6267728 |           1491630 |             745994 |                  1 |        11184809 |
  11184525
```

* Access to `reported` table during vacuuming

```
postgres=# explain(analyze, buffers, format text) select count(*) from reported;

 Finalize Aggregate  (cost=4402283.71..4402283.72 rows=1 width=8) (actual time=62663.754..62679.376 rows=1 loops=1)
   Buffers: shared hit=60496 read=3202143 written=3
   ->  Gather  (cost=4402283.50..4402283.71 rows=2 width=8) (actual time=62663.735..62679.361 rows=3 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         Buffers: shared hit=60496 read=3202143 written=3
         ->  Partial Aggregate  (cost=4401283.50..4401283.51 rows=1 width=8) (actual time=62605.754..62605.756 rows=1 loops=3)
               Buffers: shared hit=60496 read=3202143 written=3
               ->  Parallel Index Only Scan using updated_at_desc_idx on reported  (cost=0.57..4303340.24 rows=39177303 width=0) (actual time=4.406..59423.974
rows=15677300 loops=3)
                     Heap Fetches: 47032112
                     Buffers: shared hit=60496 read=3202143 written=3
 Planning Time: 0.176 ms
 Execution Time: 62679.443 ms
                 ^^^^^^^^^^^^
```

- one minute just to retrieve number of records!? 

After vacuuming
---------------

* Let's look at # records again

```
postgres=# explain(analyze, buffers, format text) select count(*) from reported;

 Finalize Aggregate  (cost=793066.38..793066.39 rows=1 width=8) (actual time=1719.988..1728.055 rows=1 loops=1)
   Buffers: shared hit=315 read=38090
   ->  Gather  (cost=793066.17..793066.38 rows=2 width=8) (actual time=1719.940..1728.046 rows=3 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         Buffers: shared hit=315 read=38090
         ->  Partial Aggregate  (cost=792066.17..792066.18 rows=1 width=8) (actual time=1712.611..1712.612 rows=1 loops=3)
               Buffers: shared hit=315 read=38090
               ->  Parallel Index Only Scan using updated_at_desc_idx on reported  (cost=0.56..742827.94 rows=19695290 width=0) (actual time=2.758..1002.403 ro
ws=15677300 loops=3)
                     Heap Fetches: 0
                     Buffers: shared hit=315 read=38090
 Planning:
   Buffers: shared hit=36
 Planning Time: 1.282 ms
 Execution Time: 1728.156 ms
(15 rows)
```

* we are back at good numbers



After some CCX Notification Service runs
========================================
* Table size is ~ same in terms of #records
    - but many pages are in 'deleted' state
    - needs defragmentation
* But vacuuming is usually not finished in time
    - it can be proved easily:

```
postgres=# SELECT pg_size_pretty(pg_total_relation_size('reported'));
 pg_size_pretty
----------------
 141 GB
```



Possible solutions for this problem
===================================
* Database partitioning
    - vertical
    - horizontal
* Reduce number of records
    - change in process logic
* Reduce size of records
    - less IOPS in overall
    - less network transfers
    - faster vacuuming
    - increase #ops done in "burst mode"

Vertical partitioning
---------------------
* Done by column
    - in our case by `report` column
    - no specific syntax in PostgreSQL
    - query usually consists of several `JOIN`s
* Won't be super useful in our case
    - `INSERT` needs to change `report` column + other columns as well
    - `SELECT` needs to query `report` column + other columns as well
* Conclusion
    - not planned to be added in near future

Horizontal partitioning
-----------------------
* Creating tables with fewer rows
    - additional tables to store the remaining rows
    - specific syntax in PostgreSQL 10.x
* When
    - old data (not used much) vs new data
    - partitioned "naturally" by country, date range etc.
* And this is our case!
    - head: new data added recently
    - body: data with age > 25 minutes and < 8 days
    - tail: data with age >= 8 days
* Conclusion
    - possible future spike

Number of records reduction
---------------------------
* We should do an `INSERT` + `on conflict do update`
    - do not add a new row to the DB for each report that is not re-notified on each run
* Calculation
    - we are processing roughly 65k reports per run, and notifying something in the range of hundreds
    - that's an estimated 64k rows with not sent same state rows added per run (every 15 minutes)
* Conclusion
    - possible future spike

Size of records reduction
-------------------------
* What's stored in `reported.report` column after all?
    - well, some JSON of "any" size
    - some stats

```
postgres=# select min(length(report)), max(length(report)), avg(length(report)) from reported;
 min | max  |         avg
-----+------+----------------------
 421 | 8341 | 426.3675676374453857
(1 row)
```

* -> seems like most reports are of similar size 421 chars

* Empty report:

```json
{
  "analysis_metadata": {
    "start": "2022-08-18T14:14:02.947224+00:00",
    "finish": "2022-08-18T14:14:03.826569+00:00",
    "execution_context": "ccx_ocp_core.context.InsightsOperatorContext",
    "plugin_sets": {
      "insights-core": {
        "version": "insights-core-3.0.290-1",
        "commit": "placeholder"
      },
      "ccx_rules_ocp": {
        "version": "ccx_rules_ocp-2022.8.17-1",
        "commit": null
      },
      "ccx_ocp_core": {
        "version": "ccx_ocp_core-2022.8.17-1",
        "commit": null
      }
    }
  },
  "reports": []
}
```


* Non empty report:
- at most 8341 characters long
- [It's huge](https://github.com/RedHatInsights/insights-results-aggregator-data/blob/master/messages/normal/05_rules_hits.json)

* What's really needed for CCX Notification Service to work?

```go
func issuesEqual(issue1, issue2 types.ReportItem) bool {
        if issue1.Type == issue2.Type &&
                issue1.Module == issue2.Module &&
                issue1.ErrorKey == issue2.ErrorKey &&
                bytes.Equal(issue1.Details, issue2.Details) {
                return true
        }
        return false
}
```

Conclusion
----------
* 421 chars/bytes are stored for all reports, even empty ones
    - 21 GB of "garbage" for 50MB reports
    - out of circa 26 GB -> ~80% of the overall size!
    - can be droped before DB write (PR already reviewed)
* Most of `report` attributes are not used and can be dropped as well
    - tags
    - links
    - ~320 chars/bytes per non-empty reports
    - ~3GB of "garbage"

Further shrinking
-----------------
* TOAST
* gzip `report.reported` column programmatically

TOAST
-----
* Compression on DB side
* Can be configured per column
* Citing official doc: ""the best thing since sliced bread"
* Fast but not too efficient
    - need to lower the threshold from 2048 chars/bytes to minimum 128

```
postgres=# alter table reported set(toast_tuple_target=128);
ALTER TABLE
```

Gzipping `report.reported`
--------------------------
* Very unpopular in academia
    - it's DB problem how/if to compress data
* In the real world
    - very easy to scale our service (thanks OpenShift)
    - hard/costly/lack of knowledge how to scale DB properly
* Questions to answer
    - will be it hard to implement?
    - will be efficient?
    - will be fast or slow?
* Will be it hard to implement?
    - basically 10 lines of code

```go
package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
)

func gzipBytes(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(src)
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func gzipString(src string) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write([]byte(src))
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
```

* Will it be efficient?
    - let's measure for "empty" and largest report

```
Filename: data/empty_report.json
String: 422  Zipped out: 252  Stripped: 40%
String: 422  Zipped out: 252  Stripped: 40%

Filename: data/large_report.json
String: 8395  Zipped out: 2454  Stripped: 71%
String: 8395  Zipped out: 2454  Stripped: 71%
```


* Will it be fast or slow?
    - forget academia, benchmarks are there to prove

```go
package main_test

import (
	"bytes"
	"compress/gzip"
	"os"

	"testing"
)

var (
	text1 string = ""
	text2 string
	text3 string
)

func init() {
	content, err := os.ReadFile("../data/empty_report.json")
	if err != nil {
		panic(err)
	}

	text2 = string(content)

	content, err = os.ReadFile("../data/large_report.json")
	if err != nil {
		panic(err)
	}

	text3 = string(content)
}

func gzipBytes(src *[]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(*src)
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func gzipString(src *string) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write([]byte(*src))
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func BenchmarkGzipEmptyString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gzipString(&text1)
	}
}

func BenchmarkGzipSimpleJSONAsString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gzipString(&text2)
	}
}

func BenchmarkGzipRuleReportAsString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gzipString(&text3)
	}
}

func BenchmarkGzipZeroBytes(b *testing.B) {
	bytes1 := []byte(text1)

	for i := 0; i < b.N; i++ {
		gzipBytes(&bytes1)
	}
}

func BenchmarkGzipSimpleJSONAsBytes(b *testing.B) {
	bytes2 := []byte(text2)

	for i := 0; i < b.N; i++ {
		gzipBytes(&bytes2)
	}
}

func BenchmarkGzipRuleReportAsBytes(b *testing.B) {
	bytes3 := []byte(text3)

	for i := 0; i < b.N; i++ {
		gzipBytes(&bytes3)
	}
}
```

* Results

```
goos: linux
goarch: amd64
pkg: main/benchmark
cpu: Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz
BenchmarkGzipEmptyString-8                153459            111102 ns/op
BenchmarkGzipSimpleJSONAsString-8          89157            143929 ns/op
BenchmarkGzipRuleReportAsString-8          35571            324360 ns/op
BenchmarkGzipZeroBytes-8                  103700            123996 ns/op
BenchmarkGzipSimpleJSONAsBytes-8           94760            157709 ns/op
BenchmarkGzipRuleReportAsBytes-8           39266            322264 ns/op
PASS
ok      main/benchmark  92.980s
```

Summary
=======
* Performance problems with `reported` table are caused by
    - high number of records stored (50M)
    - large records, especially in column `reported.report`
* Possible solution discussed there
    - vertical partitioning
        - N/A, not useful
    - horizontal partitioning
        - spike for future research
    - reduction of number of records
        - viable solution
        - spike for future research
    - reduction size of records
        - low hanging fruit - able to reduce **by** 80% easily
        - well understood
        - set of tasks to be prepared
        - to be decided: whether to do gzipping on client side
