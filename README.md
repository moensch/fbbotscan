[![GoDoc](https://godoc.org/github.com/moensch/fbbotscan?status.svg)](https://godoc.org/github.com/moensch/fbbotscan)

# fbbotscan

An experimental approach to detecting Facebook bots (actually, just a facebook data pipeline)

## Goals

To provide a public API which can identify Facebook users as bots/trolls.

This project is nowhere near there.

## Status

This is not some fancy ML approach to detecting bots, it is actually simply a data
pipeline to stream public facebook posts, comments, and sub-comments.

There is a classifier, but it simply uses an [ElasticSearch MLT Query](https://www.elastic.co/guide/en/elasticsearch/reference/5.5/query-dsl-mlt-query.html)
to find similar comments. As of now, the classifier doesn't do anything with that information yet.

## Architecture

The data pipeline consists of four major components:

* scheduler: Queries postgres for objects which need to be
scheduled to be queried for new posts or comments
* fetcher: Goes off to Facebook and pulls down new posts and comments
using the Graph API.
* storer: Stores/Indexes new Facebook comments in ElasticSearch
* classifier: Attempts to classify each new comment to determine whether
the author should be marked as a bot or not

The postgres database simply holds metadata (object IDs and time stamp of last check)
for the purpose of telling the scheduler when to issue a re-check.

All these components communicate by means of AMQP queues (tested against RabbitMQ 3.x).
In an actual setup, you would run one scheduler, and then many fetchers, storers (I should
rename this to "indexer"...) and classifiers.

The "storer" indexes new comments in ElasticSearch.

![architecture diagram](https://github.com/moensch/fbbotscan/raw/master/diagram.jpeg)

## Usage

TODO
