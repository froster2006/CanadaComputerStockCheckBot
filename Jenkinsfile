pipeline {
    agent any

    environment {
        DOCKER_IMAGE = 'froster2006/rfdchecker'
        DOCKER_CREDENTIALS_ID = 'docker-hub-credentials-dev'
    }

    stages {

        stage('Build Docker Image') {
            steps {
                script {
                    dockerImage = docker.build("${DOCKER_IMAGE}:${env.BUILD_NUMBER}")
                }
            }
        }

        stage('Push to Docker Hub') {
            steps {
                script {
                    docker.withRegistry('https://index.docker.io/v1/', "${DOCKER_CREDENTIALS_ID}") {
                        dockerImage.push()
                        dockerImage.push("latest") // optional
                    }
                }
            }
        }
    }

    post {
        success {
            echo "Docker image ${DOCKER_IMAGE}:${env.BUILD_NUMBER} pushed successfully."
        }
        failure {
            echo "Build failed."
        }
    }
}
