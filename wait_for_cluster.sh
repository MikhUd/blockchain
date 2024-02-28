#!/bin/bash

# Функция для проверки доступности кластера
wait_for_cluster() {
    echo "Waiting for the cluster to start..."
    while ! nc -z cluster 8080; do
        sleep 1
    done
    echo "Cluster is ready!"
}

# Вызываем функцию ожидания кластера
wait_for_cluster

# Запускаем ноду
echo "Starting the node..."
exec "$@"
