---
layout: default
title: Template renderer
nav_order: 7
---

# Template renderer

Messages (recommendations) that are send to ServiceLog are based on templates
read from [Content
Service](https://github.com/RedHatInsights/insights-content-service) that are
interpolated by data produced by CCX Data Pipeline (based on OCP rules). The
interpolation (applying data to template) are done within Template renderer
service that is accessible via REST API.

The [/rendered-reports](https://github.com/RedHatInsights/insights-content-template-renderer#post-rendered-reports) REST API endpoint is used

Two fields needs to be filled using the data mentioned above:

1. "summary"
1. "description"

## "summary" field

Content of this field is read from "reason" field returned in Template
renderer's response. If the length of text is longer than 255 characters (UTF-8
glyphs), it is trimmed to this value.

## "description" field

Content of this field is read from "description" field returned in Template
renderer's response. If the length of text is longer than 4000 characters
(UTF-8 glyphs), it is trimmed to this value.

Please look into [Template renderer repository](https://github.com/RedHatInsights/insights-content-template-renderer) for more info.
