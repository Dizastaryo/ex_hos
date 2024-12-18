import 'package:flutter/material.dart';
import '../models/box_model.dart';
import 'payment_screen.dart';

class BoxScreen extends StatelessWidget {
  final BoxModel box;

  const BoxScreen({super.key, required this.box});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Детали коробки: ${box.id}'),
        centerTitle: true,
        backgroundColor: const Color(0xFF4CAF50),
        elevation: 4,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(bottom: Radius.circular(16)),
        ),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Box details
            _buildBoxDetails(),
            const SizedBox(height: 20),

            // Tariff selection
            const Text(
              'Выберите тариф:',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 10),
            _buildTariffOption(
              context,
              duration: '1 месяц',
              price: _getPrice(box.type, 1),
            ),
            _buildTariffOption(
              context,
              duration: '2 месяца',
              price: _getPrice(box.type, 2),
            ),
            _buildTariffOption(
              context,
              duration: '3 месяца (10% скидка)',
              price: _getPrice(box.type, 3),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBoxDetails() {
    return Container(
      padding: const EdgeInsets.all(16.0),
      decoration: BoxDecoration(
        color: const Color(0xFFE8F5E9),
        borderRadius: BorderRadius.circular(12),
        boxShadow: const [
          BoxShadow(
            color: Colors.black26,
            blurRadius: 8,
            offset: Offset(0, 4),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Коробка: ${box.id}',
            style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            'Размер: ${box.width} x ${box.height} см',
            style: const TextStyle(fontSize: 16),
          ),
          const SizedBox(height: 8),
          Text(
            'Тип: ${box.type.name.toUpperCase()}',
            style: const TextStyle(fontSize: 16),
          ),
        ],
      ),
    );
  }

  double _getPrice(BoxType type, int months) {
    const prices = {
      BoxType.xxs: [8000.0, 15000.0, 21600.0],
      BoxType.xs: [3500.0, 6500.0, 9450.0],
      BoxType.s: [10000.0, 19000.0, 27000.0],
      BoxType.m: [13000.0, 24000.0, 34200.0],
      BoxType.l: [15000.0, 28000.0, 40500.0],
      BoxType.xl: [30000.0, 56000.0, 81000.0],
    };

    final priceList = prices[type];
    if (priceList == null || months < 1 || months > priceList.length) {
      return 0.0;
    }
    return priceList[months - 1];
  }

  Widget _buildTariffOption(BuildContext context,
      {required String duration, required double price}) {
    return GestureDetector(
      onTap: () {
        Navigator.push(
          context,
          MaterialPageRoute(
            builder: (context) => PaymentScreen(
              box: box,
              duration: duration,
              price: price,
            ),
          ),
        );
      },
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 8.0),
        padding: const EdgeInsets.all(16.0),
        decoration: BoxDecoration(
          color: const Color(0xFFF1F8E9),
          borderRadius: BorderRadius.circular(12),
          boxShadow: const [
            BoxShadow(
              color: Colors.black12,
              blurRadius: 6,
              offset: Offset(0, 3),
            ),
          ],
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              duration,
              style: const TextStyle(fontSize: 16),
            ),
            Text(
              '${price.toStringAsFixed(2)} ₽',
              style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
            ),
          ],
        ),
      ),
    );
  }
}
