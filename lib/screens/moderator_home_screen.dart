import 'package:flutter/material.dart';
import 'add_product_screen.dart';
import 'orders_screen.dart';
import 'products_screen.dart';

class ModeratorHomeScreen extends StatelessWidget {
  const ModeratorHomeScreen({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Экран модератора'),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: const [
            Text(
              'Добро пожаловать, модератор!',
              style: TextStyle(fontSize: 24),
            ),
            SizedBox(height: 20),
            Text(
              'Здесь вы можете управлять товарами, заказами и отзывами.',
              style: TextStyle(fontSize: 16),
            ),
          ],
        ),
      ),
      bottomNavigationBar: BottomNavigationBar(
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.shop),
            label: 'Продукты',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.list),
            label: 'Заказы',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.add),
            label: 'Добавить товар',
          ),
        ],
        onTap: (int index) {
          if (index == 0) {
            Navigator.push(
              context,
              MaterialPageRoute(builder: (context) => const ProductsScreen()),
            );
          } else if (index == 1) {
            Navigator.push(
              context,
              MaterialPageRoute(builder: (context) => const OrdersScreen()),
            );
          } else if (index == 2) {
            Navigator.push(
              context,
              MaterialPageRoute(builder: (context) => const AddProductScreen()),
            );
          }
        },
      ),
    );
  }
}
