package com.aionemu.gameserver.utils.idfactory;

import java.util.concurrent.atomic.AtomicInteger;

/**
 * Test-classpath stand-in for the database-backed IDFactory (test classes come first on the surefire classpath), so
 * the quest trace harness can create items without a database.
 */
public class IDFactory
{
	private static final IDFactory	instance	= new IDFactory();
	private final AtomicInteger		next		= new AtomicInteger(0x100000);

	public static final IDFactory getInstance()
	{
		return instance;
	}

	public int nextId()
	{
		return next.getAndIncrement();
	}

	public void lockIds(Iterable<Integer> ids)
	{
	}

	public void releaseId(int id)
	{
	}

	public int getUsedCount()
	{
		return next.get() - 0x100000;
	}
}
