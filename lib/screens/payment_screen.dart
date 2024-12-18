import 'package:flutter/material.dart';
import '../models/box_model.dart';

class PaymentScreen extends StatefulWidget {
  final BoxModel box;
  final String duration;
  final double price;

  const PaymentScreen({
    super.key,
    required this.box,
    required this.duration,
    required this.price,
  });

  @override
  State<PaymentScreen> createState() => _PaymentScreenState();
}

class _PaymentScreenState extends State<PaymentScreen> {
  final _formKey = GlobalKey<FormState>();
  final TextEditingController cardNumberController = TextEditingController();
  final TextEditingController cardHolderController = TextEditingController();
  final TextEditingController expiryDateController = TextEditingController();
  final TextEditingController cvvController = TextEditingController();

  @override
  void dispose() {
    cardNumberController.dispose();
    cardHolderController.dispose();
    expiryDateController.dispose();
    cvvController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Оплата'),
        backgroundColor: const Color(0xFF4CAF50),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: SingleChildScrollView(
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Информация о выбранной коробке и тарифе
                Text(
                  'Выбранная коробка: ${widget.box.id}',
                  style: const TextStyle(
                      fontSize: 20, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 10),
                Text(
                  'Тариф: ${widget.duration} за ${widget.price.toStringAsFixed(2)} ₽',
                  style: const TextStyle(fontSize: 16),
                ),
                const SizedBox(height: 20),

                // Поля для ввода данных карты
                _buildTextField(
                  'Номер карты',
                  TextInputType.number,
                  controller: cardNumberController,
                  validator: (value) {
                    if (value == null || value.isEmpty) {
                      return 'Введите номер карты';
                    }
                    if (value.length < 16) {
                      return 'Номер карты должен содержать 16 цифр';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 16),
                _buildTextField(
                  'Имя владельца карты',
                  TextInputType.name,
                  controller: cardHolderController,
                  validator: (value) {
                    if (value == null || value.isEmpty) {
                      return 'Введите имя владельца';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 16),
                _buildTextField(
                  'Срок действия (MM/YY)',
                  TextInputType.datetime,
                  controller: expiryDateController,
                  validator: (value) {
                    if (value == null || value.isEmpty) {
                      return 'Введите срок действия карты';
                    }
                    if (!RegExp(r'^(0[1-9]|1[0-2])\/\d{2}$').hasMatch(value)) {
                      return 'Введите срок действия в формате MM/YY';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 16),
                _buildTextField(
                  'CVV',
                  TextInputType.number,
                  controller: cvvController,
                  validator: (value) {
                    if (value == null || value.isEmpty) {
                      return 'Введите CVV код';
                    }
                    if (value.length != 3) {
                      return 'CVV код должен содержать 3 цифры';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 32),

                // Кнопка оплаты
                ElevatedButton(
                  onPressed: () {
                    if (_formKey.currentState?.validate() ?? false) {
                      // Логика оплаты
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                            content: Text('Оплата успешно завершена!')),
                      );
                      Navigator.pop(context); // Возврат на предыдущий экран
                    } else {
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                            content: Text('Проверьте введенные данные.')),
                      );
                    }
                  },
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF4CAF50),
                    padding: const EdgeInsets.symmetric(vertical: 16.0),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(16),
                    ),
                  ),
                  child: const Center(
                    child: Text(
                      'Оплатить',
                      style: TextStyle(fontSize: 18, color: Colors.white),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  // Вспомогательная функция для создания текстовых полей
  Widget _buildTextField(
    String label,
    TextInputType keyboardType, {
    required TextEditingController controller,
    String? Function(String?)? validator,
  }) {
    return TextFormField(
      controller: controller,
      keyboardType: keyboardType,
      decoration: InputDecoration(
        labelText: label,
        border: const OutlineInputBorder(),
        contentPadding: const EdgeInsets.symmetric(horizontal: 10),
      ),
      validator: validator,
    );
  }
}
