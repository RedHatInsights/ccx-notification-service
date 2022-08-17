# Events filter in CCX Notification Service

## Why?

* to be able to decide which events to process and send
    - to notification service
    - to service log

* configurable w/o the need to change the code
    - and flexible enough

* OTOH current situation

```go
        // TODO: make this configurable via config file
        totalRiskThreshold = 3
```



## Desirable situation

* Event filtering configurable via TOML files and environment variables

```
[kafka_broker]
enabled = true
address = "kafka:29092"
...
...
...
likelihood_threshold = 0
impact_threshold = 0
severity_threshold = 0
total_risk_threshold = 3
event_filter = "totalRisk >= totalRiskThreshold"

[service_log]
enabled = false
access_token = ""
url = "https://api.openshift.com/api/service_logs/v1/cluster_logs/"
...
...
...
likelihood_threshold = 0
impact_threshold = 3
severity_threshold = 0
total_risk_threshold = 3
event_filter = "totalRisk > totalRiskThreshold && severity > 2*(severityTreshold + impact)"
```



## How?

* DSL
    - minimal
    - easy to comprehend
    - safe!!!
    - isolated
    - ideally compatible with Go (operator precedence etc.)
* Safe?
    - no loops
    - no indexing
    - no access to variables etc.
    - unable to call any function
    - just selected 'named values'
    - -> no needs to care about sandboxing



## Implementation

* Expression language
    - standard operator precedence (Go-compatible)
    - parenthesis etc.
* Operators
    - arithmetic
    - logic
    - relational
* Values
    - selected values only (like 'severity')
    - explicitly passed via map in code



## Implementation

* Three stages
    - expression tokenization
    - transform: algebraic -> RPN (postfix)
    - RPN (postfix) evaluation



### Expression tokenization

* Based on standard Go lexer
    - so trivial to implement
    - and super safe
* Our implementation
    - Four lines of code
    - [Source code](https://github.com/RedHatInsights/ccx-notification-service/blob/66f8adad4369efbeaa5a1582b11d64053407d007/docs/demos/dsl-events-filtering/events_filter.go#L32)



### Transform: algebraic -> RPN

* The famous shunting yard algorithm
    - [Wikipedia article](https://en.wikipedia.org/wiki/Shunting_yard_algorithm)
* Our implementation
    - 100 lines of code including comments
    - [Source code](https://github.com/RedHatInsights/ccx-notification-service/blob/master/docs/demos/dsl-events-filtering/shunting_yard.go#L28)



### RPN evaluation

* Stack based, trivial to implement
    - [Wikipedia article](https://en.wikipedia.org/wiki/Polish_notation#Evaluation_algorithm)
* Our implementation
    - 60 lines of code including comments
    - [Source code](https://github.com/RedHatInsights/ccx-notification-service/blob/master/docs/demos/dsl-events-filtering/evaluator.go#L43)



### Values mapping

* Via dictionary (a map)
* No other named values can be used
* Super safe
    - no custom (usually broken) sandboxing needed
* Our implementation
    - 1 line of code + one extra line for each value to be passed
    - [Source code](https://github.com/RedHatInsights/ccx-notification-service/blob/master/docs/demos/dsl-events-filtering/events_filter.go#L50)



## Links
* [Shunting yard algorithm](https://en.wikipedia.org/wiki/Shunting_yard_algorithm)
* [Polish notation](https://en.wikipedia.org/wiki/Polish_notation)
* [Reverse Polish notation](https://en.wikipedia.org/wiki/Reverse_Polish_notation)
