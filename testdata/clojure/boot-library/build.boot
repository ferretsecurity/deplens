(def release "0.1.0")

(set-env!
 :dependencies '[; test support is declared with the production library
                [demo/library "4.5.6"]
                [demo/test-runner "4.5.7-SNAPSHOT" :scope "test"]])
