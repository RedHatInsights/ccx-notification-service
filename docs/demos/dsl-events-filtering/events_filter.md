# Events filter

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

## How?

* DSL
    - minimal
    - easy to comprehend
    - safe!!!
    - isolated
* Safe?
    - no loops
    - no indexing
    - no access to variables etc.
    - just selected 'named values'



## Implementation

* Expression language
    - std. precedence
    - parenthesis etc.
* Operators
    - arithmetic
    - logic
    - relational
* Values
    - selected values only (like 'severity')



## Implementation

* Two stages
    - transform: algebraic -> RPN
    - RPN evaluation

### Transform: algebraic -> RPN

* The famous shunting yard algorithm

### RPN evaluation

* Stack based, trivial to implement

### Values mapping

* Via dictionary
* No other named values can be used
