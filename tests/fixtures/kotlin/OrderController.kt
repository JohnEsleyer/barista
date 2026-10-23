package com.example.service

import barista.spring.annotation.RestController
import barista.spring.annotation.RequestMapping
import barista.spring.annotation.GetMapping
import barista.spring.annotation.PostMapping

@RestController
@RequestMapping("/api")
class OrderController {

    @GetMapping("/status")
    fun getStatus(): String {
        return "Orders service healthy"
    }

    @PostMapping("/submit")
    fun submitOrder(orderData: String): String {
        return "Order received: $orderData"
    }
}
