package com.example.service;

import barista.spring.annotation.RestController;
import barista.spring.annotation.RequestMapping;
import barista.spring.annotation.GetMapping;
import barista.spring.annotation.PostMapping;

@RestController
@RequestMapping("/api")
public class OrderController {

    @GetMapping("/status")
    public String getStatus() {
        return "Orders service healthy";
    }

    @PostMapping("/submit")
    public String submitOrder(String orderData) {
        StringBuilder sb = new StringBuilder();
        sb.append("Order received: ");
        sb.append(orderData);
        return sb.toString();
    }
}
