<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Third Party Services
    |--------------------------------------------------------------------------
    |
    | This file is for storing the credentials for third party services such
    | as Resend, Postmark, AWS, and more. This file provides the de facto
    | location for this type of information, allowing packages to have
    | a conventional file to locate the various service credentials.
    |
    */

    'postmark' => [
        'key' => env('POSTMARK_API_KEY'),
    ],

    'resend' => [
        'key' => env('RESEND_API_KEY'),
    ],

    'ses' => [
        'key' => env('AWS_ACCESS_KEY_ID'),
        'secret' => env('AWS_SECRET_ACCESS_KEY'),
        'region' => env('AWS_DEFAULT_REGION', 'us-east-1'),
    ],

    'slack' => [
        'notifications' => [
            'bot_user_oauth_token' => env('SLACK_BOT_USER_OAUTH_TOKEN'),
            'channel' => env('SLACK_BOT_USER_DEFAULT_CHANNEL'),
        ],
    ],

    'rabbitmq' => [
        'host' => env('RABBITMQ_HOST', '127.0.0.1'),
        'port' => env('RABBITMQ_PORT', 5673),
        'user' => env('RABBITMQ_USER', 'dev_rabbitmq'),
        'password' => env('RABBITMQ_PASSWORD', 'dev_rabbitmq_password'),
        'vhost' => env('RABBITMQ_VHOST', '/'),
        'connection_timeout' => env('RABBITMQ_CONNECTION_TIMEOUT', 3.0),
        'read_write_timeout' => env('RABBITMQ_READ_WRITE_TIMEOUT', 3.0),
        'heartbeat' => env('RABBITMQ_HEARTBEAT', 30),
        'publisher_confirm_timeout' => env('RABBITMQ_PUBLISHER_CONFIRM_TIMEOUT', 5.0),
        'order_created_queue' => env('RABBITMQ_ORDER_CREATED_QUEUE', 'commerce.order.created'),
        'order_created_dead_letter_queue' => env('RABBITMQ_ORDER_CREATED_DLQ', 'commerce.order.created.dlq'),
    ],

];
