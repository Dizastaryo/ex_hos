import 'package:flutter/material.dart';
import 'about_screen.dart';
import 'admin_orders_screen.dart';
import 'profile_screen.dart';

class ModeratorHomeScreen extends StatefulWidget {
  const ModeratorHomeScreen({Key? key}) : super(key: key);

  @override
  _ModeratorHomeScreenState createState() => _ModeratorHomeScreenState();
}

class _ModeratorHomeScreenState extends State<ModeratorHomeScreen> {
  int _currentPage = 0;

  final List<Widget> _pages = [
    AdminOrdersScreen(),
    AboutScreen(),
    ProfileScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Панель модератора'),
        centerTitle: true,
        backgroundColor: Colors.green,
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: _pages[_currentPage],
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentPage,
        selectedItemColor: Colors.green,
        unselectedItemColor: Colors.grey,
        showUnselectedLabels: true,
        type: BottomNavigationBarType.fixed,
        onTap: (index) {
          setState(() {
            _currentPage = index;
          });
        },
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.shop_outlined),
            activeIcon: Icon(Icons.shop),
            label: 'Заказы',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.list_alt_outlined),
            activeIcon: Icon(Icons.list_alt),
            label: 'Управление',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.account_circle_outlined), // Современная иконка
            activeIcon: Icon(Icons.account_circle),
            label: 'Профил',
          ),
        ],
      ),
    );
  }
}
