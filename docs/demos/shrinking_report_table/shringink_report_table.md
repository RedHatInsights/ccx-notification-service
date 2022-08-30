Optimizing and shrinking `reported` table for CCX Notification Service
====================================================================

Reasons
-------
* Access to `reported` table is very slow on prod
* Simple `select count(*) from report` can take minutes!
* Table grows over time (new features) up to ~150GB as higher limit today
* Will be limiting factor for integration with ServiceLog

Table structure
---------------
* Seems simple enough
    - and it is
    - but devil is in the details as usual

```
```

Table size
----------
* CCX Notification Service is started with 15 minutes frequence
    - i.e. 4 times per hour
* On each run approximatelly 65000 reports are written into the table
* Reports are stored for 8 days
    - limit required for 'cooldown' feature
* Simple math
    - number of records=8 days × 24 hours × 4 (runs per hour) × 65000
    - number of records=8×24×4×65000
    - 49920000 ~ 50 millions!
* => "each bytes in reports counts!"



Original table
==============

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
* OTOH for approximatelly 50M records it is still ok
* Can be make faster
     - by introducing simpler index (ID)
     - and by increasing table size by 50 000 000 * sizeof(ID) :)

Table as live entity in the system
==================================
* Content of `reported` table is changing quite rapidly
    - well it depends how we look at it
    - each hour, approximatelly 65000 × 4 = 260000 reports is created/deleted
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
     - this is the little devil

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
* But vacuuming is usually not finished in time

```
postgres=# SELECT pg_size_pretty(pg_total_relation_size('reported'));
 pg_size_pretty
----------------
 141 GB
```
