import groovy.json.JsonSlurper

def k8sVersions = ["1.12", "1.13", "1.14", "1.15", "1.16"]
def clusterConfigurations = ["default-config"]
def tasks = [:]

stage("Before") {
    node {
        echo "run this static stuff before the matrix"
    }
}

for(int i=0; i< k8sVersions.size(); i++) {
    def version = k8sVersions[i]
    for(int j=0; j< clusterConfigurations.size(); j++) {
        def clusterConfig = clusterConfigurations[j]
        tasks["${version}/${clusterConfig}"] = {
            stage("cluster create") {
                node {
                    checkout scm
                    def inputFile = new File('./test/e2e/cluster.json')
                    def inputJSON = new JsonSlurper().parse(inputFile)
                    withEnv(["ORCHESTRATOR_RELEASE=${version}"]) {
                        withCredentials([string(credentialsId: 'AKS_ENGINE_TENANT_ID', variable: 'TENANT_ID'), string(credentialsId: 'AKS_ENGINE_3014546b_CLIENT_ID', variable: 'CLIENT_ID'), string(credentialsId: 'AKS_ENGINE_3014546b_CLIENT_SECRET', variable: 'CLIENT_SECRET')]) {
                            sh "./test/e2e/cluster.sh"
                        }
                    }
                }
            }
        }
    }
}

stage ("Matrix") {
    parallel tasks
}

stage("After") {
    node {
        echo "run this static stuff after the entire"
    }
}