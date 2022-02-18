@Library('dst-shared@master') _

dockerBuildPipeline {
        githubPushRepo = "Cray-HPE/hms-compcredentials"
        repository = "cray"
        imagePrefix = "hms"
        app = "compcredentials"
        name = "hms-compcredentials"
        description = "Cray HMS compcredentials code."
        dockerfile = "Dockerfile"
        slackNotification = ["", "", false, false, true, true]
        product = "internal"
}
