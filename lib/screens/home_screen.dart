import 'package:flutter/material.dart';
import 'about_screen.dart'; // Экран "О компании"
import 'profile_screen.dart'; // Экран профиля
import 'my_rentals_screen.dart'; // Экран "Мои аренды"
import 'notifications_screen.dart'; // Экран уведомлений
import 'support_screen.dart'; // Экран "Поддержка"

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
    SupportScreen(),
    ProfileScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        centerTitle: true,
        title: Row(
          children: [
            Container(
              decoration: BoxDecoration(
                color: Colors.white, // Белый фон для круга
                shape: BoxShape.circle,
                boxShadow: [
                  BoxShadow(
                    color: Colors.grey.withOpacity(0.2),
                    spreadRadius: 2,
                    blurRadius: 5,
                  ),
                ],
              ),
              child: Padding(
                padding: const EdgeInsets.all(8.0),
                child: Image.asset(
                  'assets/amanzat_logo.png',
                  height: 30,
                ),
              ),
            ),
            const SizedBox(width: 10),
            const Text(
              'SANDYQ',
              style: TextStyle(
                fontWeight: FontWeight.bold,
                color: Colors.black,
                fontSize: 20,
              ),
            ),
          ],
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.notifications_outlined, color: Colors.black),
            onPressed: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => const NotificationsScreen(),
                ),
              );
            },
          ),
        ],
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: _pages[_currentPage],
      ),
      bottomNavigationBar: BottomNavigationBar(
        items: [
          BottomNavigationBarItem(
            icon: Icon(Icons.dashboard_outlined), // Современная иконка
            activeIcon: Icon(Icons.dashboard), // Активная версия
            label: 'Главная',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.assignment_outlined), // Современная иконка
            activeIcon: Icon(Icons.assignment), // Активная версия
            label: 'Мои аренды',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.support_agent_outlined), // Современная иконка
            activeIcon: Icon(Icons.support_agent), // Активная версия
            label: 'Поддержка',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.account_circle_outlined), // Современная иконка
            activeIcon: Icon(Icons.account_circle), // Активная версия
            label: 'Профиль',
          ),
        ],
        currentIndex: _currentPage,
        selectedItemColor:
            const Color.fromARGB(255, 34, 146, 34), // Цвет активной иконки
        unselectedItemColor:
            const Color.fromARGB(255, 65, 65, 65), // Цвет неактивных иконок
        showUnselectedLabels: true,
        type: BottomNavigationBarType.fixed,
        onTap: (index) {
          setState(() {
            _currentPage = index;
          });
        },
      ),
    );
  }
}
