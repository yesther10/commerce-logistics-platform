<?php

use App\Http\Controllers\Api\V1\OrderController;
use App\Http\Controllers\Api\V1\ProductController;
use Illuminate\Support\Facades\Route;

Route::prefix('v1')->group(function (): void {
    Route::apiResource('products', ProductController::class)->only(['index', 'store']);
    Route::apiResource('orders', OrderController::class)->only(['store', 'show']);
});
