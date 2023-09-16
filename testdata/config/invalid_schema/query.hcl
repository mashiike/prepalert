query "hoge" {
    runner = "invalid"
}

query "fuga" {
    runner = [
        query_runner.hoge.hoge,
        query_runner.fuga.fuga,
    ]
}
