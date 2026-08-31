(defproject example/profiles "0.1.0"
  :plugins [[example/release "3.0.0"]]
  :profiles {:provided {:dependencies [[example/api "1.0.0"]]}
             :dev {:dependencies [[example/repl "2.0.0"]]}
             :repl {:dependencies [[example/worker "3.0.0"]]}})
