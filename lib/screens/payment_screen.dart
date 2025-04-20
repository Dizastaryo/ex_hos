// lib/screens/payment_screen.dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';
import 'payment_status_screen.dart';

class PaymentScreen extends StatefulWidget {
  final int orderId;
  final double orderTotal; // <-- передаём сюда сумму из OrderRead
  const PaymentScreen({required this.orderId, required this.orderTotal});

  @override
  State<PaymentScreen> createState() => _PaymentScreenState();
}

class _PaymentScreenState extends State<PaymentScreen> {
  final _formKey = GlobalKey<FormState>();
  final _cardNumberCtrl = TextEditingController();
  final _expiryCtrl = TextEditingController();
  final _cvvCtrl = TextEditingController();
  bool _isLoading = false;

  @override
  void dispose() {
    _cardNumberCtrl.dispose();
    _expiryCtrl.dispose();
    _cvvCtrl.dispose();
    super.dispose();
  }

  Future<void> _submitPayment() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isLoading = true);
    try {
      final resp = await Provider.of<OrderService>(context, listen: false)
          .createPayment(widget.orderId, widget.orderTotal);
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(
          builder: (_) => PaymentStatusScreen(orderId: widget.orderId),
        ),
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  String? _validateCardNumber(String? v) {
    if (v == null || v.trim().isEmpty) return 'Введите номер карты';
    final digits = v.replaceAll(' ', '');
    if (digits.length != 16 || int.tryParse(digits) == null) {
      return 'Неверный номер карты';
    }
    return null;
  }

  String? _validateExpiry(String? v) {
    if (v == null ||
        !RegExp(r'^(0[1-9]|1[0-2])\/?([0-9]{2})$').hasMatch(v.trim())) {
      return 'Неверный формат MM/YY';
    }
    return null;
  }

  String? _validateCvv(String? v) {
    if (v == null || v.trim().length != 3 || int.tryParse(v) == null) {
      return 'Неверный CVV';
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Оплата заказа')),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : Padding(
              padding: const EdgeInsets.all(16),
              child: Form(
                key: _formKey,
                child: Column(
                  children: [
                    TextFormField(
                      controller: _cardNumberCtrl,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Номер карты',
                        hintText: 'XXXX XXXX XXXX XXXX',
                        border: OutlineInputBorder(),
                      ),
                      validator: _validateCardNumber,
                    ),
                    const SizedBox(height: 16),
                    Row(
                      children: [
                        Expanded(
                          child: TextFormField(
                            controller: _expiryCtrl,
                            keyboardType: TextInputType.datetime,
                            decoration: const InputDecoration(
                              labelText: 'Срок (MM/YY)',
                              border: OutlineInputBorder(),
                            ),
                            validator: _validateExpiry,
                          ),
                        ),
                        const SizedBox(width: 16),
                        Expanded(
                          child: TextFormField(
                            controller: _cvvCtrl,
                            keyboardType: TextInputType.number,
                            obscureText: true,
                            decoration: const InputDecoration(
                              labelText: 'CVV',
                              border: OutlineInputBorder(),
                            ),
                            validator: _validateCvv,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 24),
                    SizedBox(
                      width: double.infinity,
                      child: ElevatedButton(
                        onPressed: _submitPayment,
                        style: ElevatedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: 16),
                          backgroundColor: Colors.green,
                        ),
                        child: const Text(
                          'Оплатить',
                          style: TextStyle(fontSize: 16),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
    );
  }
}
