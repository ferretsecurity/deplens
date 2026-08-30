(defproject example/tools "0.1.0"
  :profiles {:dev {:plugins [[example/linter "4.0.0"]]
                   :dependencies [[example/chart "5.0.0"]]}}
  :dependencies [[example/base "6.0.0"]])
