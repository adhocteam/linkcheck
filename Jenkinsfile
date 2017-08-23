pipeline {
  agent any
  parameters {
      choice(name: 'ENV', choices: 'test\nimp\nprod', description: 'One of dev, test, imp or prod')
       choice(name: 'APP',
            choices:'learn\nsep_screener\nexemptions_screener\nyoung_adults_screener\nflh\nflh_upkeep\nflh_admin',
            description: 'Application to clear'
        )

  }
  stages {
    stage('Download') {
      steps {
      sh """
        export AWS_ACCESS_KEY_ID=AKIAIP4CC5Z2HKY2JOKQ
        export AWS_SECRET_ACCESS_KEY=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
        aws s3 cp s3://linkchecker-releases/linkchecker-latest.rpm linkchecker-latest.rpm
        sudo yum remove -y linkcheck
        sudo rpm -i linkchecker-latest.rpm
        """
      }
    }
    stage("Scan") {
      steps {
        configFileProvider([configFile(fileId: '5c1baee8-461a-49d1-8308-297b16d49f6c', variable: 'configFile')]) {
          sh """
            wget -q -O jq https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64
            chmod +x ./jq
            /bin/linkcheck -root `cat "${configFile}" | ./jq '.${params.APP}.${params.ENV}[0]' | tr -d '"'`
          """
        }
      }
    }
  }
}

