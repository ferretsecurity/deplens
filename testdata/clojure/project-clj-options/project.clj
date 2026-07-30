(defproject example/options "0.1.0"
  :dependencies [[ring/ring-core "1.12.2" :exclusions [org.clojure/clojure]]
                 [com.example/library "2.0.0" :classifier "tests" :scope "provided"]])
