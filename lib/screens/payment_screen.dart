import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';
import 'payment_status_screen.dart';

/// Форматтер номера карты: 1234 5678 9012 3456
class CardNumberInputFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
      TextEditingValue oldValue, TextEditingValue newValue) {
    final digits = newValue.text.replaceAll(RegExp(r'\D'), '');
    final buffer = StringBuffer();
    for (var i = 0; i < digits.length; i++) {
      if (i != 0 && i % 4 == 0) buffer.write(' ');
      buffer.write(digits[i]);
    }
    return TextEditingValue(
      text: buffer.toString(),
      selection: TextSelection.collapsed(offset: buffer.length),
    );
  }
}

/// Форматтер срока действия карты: 10/25
class ExpiryDateFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
      TextEditingValue oldValue, TextEditingValue newValue) {
    var digits = newValue.text.replaceAll(RegExp(r'\D'), '');
    if (digits.length > 4) digits = digits.substring(0, 4);
    final buffer = StringBuffer();
    for (var i = 0; i < digits.length; i++) {
      if (i == 2) buffer.write('/');
      buffer.write(digits[i]);
    }
    return TextEditingValue(
      text: buffer.toString(),
      selection: TextSelection.collapsed(offset: buffer.length),
    );
  }
}

class PaymentScreen extends StatefulWidget {
  final int orderId;
  final double orderTotal;

  const PaymentScreen(
      {required this.orderId, required this.orderTotal, super.key});

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
      await context.read<OrderService>().createPayment(widget.orderId);
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(
            builder: (_) => PaymentStatusScreen(orderId: widget.orderId)),
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  String? _validateCardNumber(String? value) {
    final digits = value?.replaceAll(' ', '') ?? '';
    if (digits.length != 16) return 'Введите 16-значный номер карты';
    return null;
  }

  String? _validateExpiry(String? value) {
    if (value == null ||
        !RegExp(r'^(0[1-9]|1[0-2])\/\d{2}$').hasMatch(value.trim())) {
      return 'Неверный формат даты (MM/YY)';
    }
    return null;
  }

  String? _validateCvv(String? value) {
    if (value == null || value.trim().length != 3)
      return 'Введите 3-значный CVV';
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
                    _buildCardNumberField(),
                    const SizedBox(height: 16),
                    Row(
                      children: [
                        Expanded(child: _buildExpiryField()),
                        const SizedBox(width: 16),
                        Expanded(child: _buildCvvField()),
                      ],
                    ),
                    const SizedBox(height: 24),
                    _buildSubmitButton(),
                  ],
                ),
              ),
            ),
    );
  }

  Widget _buildCardNumberField() => TextFormField(
        controller: _cardNumberCtrl,
        keyboardType: TextInputType.number,
        inputFormatters: [
          FilteringTextInputFormatter.digitsOnly,
          CardNumberInputFormatter(),
        ],
        decoration: const InputDecoration(
          labelText: 'Номер карты',
          hintText: '1234 5678 9012 3456',
          border: OutlineInputBorder(),
        ),
        validator: _validateCardNumber,
      );

  Widget _buildExpiryField() => TextFormField(
        controller: _expiryCtrl,
        keyboardType: TextInputType.number,
        inputFormatters: [
          FilteringTextInputFormatter.digitsOnly,
          ExpiryDateFormatter(),
        ],
        decoration: const InputDecoration(
          labelText: 'Срок действия',
          hintText: 'MM/YY',
          border: OutlineInputBorder(),
        ),
        validator: _validateExpiry,
      );

  Widget _buildCvvField() => TextFormField(
        controller: _cvvCtrl,
        keyboardType: TextInputType.number,
        obscureText: true,
        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
        decoration: const InputDecoration(
          labelText: 'CVV',
          hintText: '123',
          border: OutlineInputBorder(),
        ),
        validator: _validateCvv,
      );

  Widget _buildSubmitButton() => SizedBox(
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
      );
}
