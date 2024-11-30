// lib/screens/home_screen.dart
import 'package:flutter/material.dart';
import 'about_screen.dart'; // Экран "О компании"
import 'profile_screen.dart'; // Экран профиля
import 'my_rentals_screen.dart'; // Экран "Мои аренды"

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  _HomeScreenState createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  var _currentPage = 0;

  final List<Widget> _pages = [
    AboutScreen(),
    MyRentalsScreen(),
    ProfileScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Row(
          children: [
            Image.asset(
              'assets/amanzat_logo.png',
              height: 30,
            ),
            const SizedBox(width: 10),
            const Text('Amanzat',
                style: TextStyle(fontWeight: FontWeight.bold)),
          ],
        ),
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: _pages[_currentPage],
      ),
      bottomNavigationBar: BottomNavigationBar(
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.home_outlined),
            label: 'Главная',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.business_center_outlined),
            label: 'Мои аренды',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.person_outlined),
            label: 'Профиль',
          ),
        ],
        currentIndex: _currentPage,
        onTap: (index) {
          setState(() {
            _currentPage = index;
          });
        },
      ),
    );
  }
}
