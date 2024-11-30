import 'package:flutter/material.dart';

class PaymentScreen extends StatelessWidget {
  final String boxId;
  final int months;
  final int price;
  final bool courierNeeded;

  const PaymentScreen({
    super.key,
    required this.boxId,
    required this.months,
    required this.price,
    required this.courierNeeded,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Оплата'),
        backgroundColor: const Color(0xFF6C9942),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Информация о выбранной коробке и тарифах
            Text(
              'Коробка: $boxId',
              style: const TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 16),
            Text(
              'Тариф: $months месяц(ев) за $price тг',
              style: const TextStyle(fontSize: 16),
            ),
            const SizedBox(height: 8),
            Text(
              'Курьер: ${courierNeeded ? "Да" : "Нет"}',
              style: const TextStyle(fontSize: 16),
            ),
            const SizedBox(height: 32),

            // Поля для ввода данных карты
            _buildTextField('Номер карты', TextInputType.number),
            const SizedBox(height: 16),
            _buildTextField('Имя владельца карты', TextInputType.name),
            const SizedBox(height: 16),
            _buildTextField('Срок действия (MM/YY)', TextInputType.datetime),
            const SizedBox(height: 16),
            _buildTextField('CVV', TextInputType.number),
            const SizedBox(height: 32),

            // Кнопка оплаты
            ElevatedButton(
              onPressed: () {
                // Логика оплаты
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('Оплата успешно завершена!')),
                );
                Navigator.pop(context); // Возврат на предыдущий экран
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFF6C9942),
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
    );
  }

  // Вспомогательная функция для создания текстовых полей
  Widget _buildTextField(String label, TextInputType keyboardType) {
    return TextField(
      keyboardType: keyboardType,
      decoration: InputDecoration(
        labelText: label,
        border: OutlineInputBorder(),
        contentPadding: const EdgeInsets.symmetric(horizontal: 10),
      ),
    );
  }
}

